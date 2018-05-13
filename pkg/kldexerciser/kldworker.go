package kldexerciser

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ecrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	log "github.com/sirupsen/logrus"
)

// Worker runs the specified number transactions the specified number of times then exits
type Worker struct {
	Index            int
	Name             string
	LoopIndex        uint64
	Exerciser        *Exerciser
	Nonce            uint64
	CompiledContract *CompiledSolidity
	RPC              *rpc.Client
	EthClient        *ethclient.Client
	Account          common.Address
	PrivateKey       *ecdsa.PrivateKey
	Signer           types.EIP155Signer
}

func (w Worker) debug(message string, inserts ...interface{}) {
	log.Debug(fmt.Sprintf("%s/%06d: ", w.Name, w.LoopIndex), fmt.Sprintf(message, inserts...))
}

func (w Worker) info(message string, inserts ...interface{}) {
	log.Info(fmt.Sprintf("%s/%06d: ", w.Name, w.LoopIndex), fmt.Sprintf(message, inserts...))
}

func (w Worker) error(message string, inserts ...interface{}) {
	log.Error(fmt.Sprintf("%s/%06d: ", w.Name, w.LoopIndex), fmt.Sprintf(message, inserts...))
}

// generateAccount generates a new account for this worker to use in external signing
func (w *Worker) generateAccount() error {
	key, err := ecrypto.GenerateKey()
	if err != nil {
		return fmt.Errorf("Generating key: %s", err)
	}
	w.PrivateKey = key
	w.Account = ecrypto.PubkeyToAddress(key.PublicKey)
	w.Signer = types.NewEIP155Signer(big.NewInt(w.Exerciser.ChainID))
	return nil
}

// signTxn externally signs a transaction
func (w *Worker) sendSignedTransaction(ctx context.Context, tx *types.Transaction) error {
	signedTx, _ := types.SignTx(tx, w.Signer, w.PrivateKey)
	var buff bytes.Buffer
	signedTx.EncodeRLP(&buff)
	from, _ := types.Sender(w.Signer, signedTx)
	w.debug("TX:%s externally signed. ChainID=%d From=%s", tx.Hash().Hex(), w.Exerciser.ChainID, from.Hex())

	return w.EthClient.SendTransaction(ctx, signedTx)
}

// sendTransaction sends a single transaction and returns the txid
func (w *Worker) sendTransaction() (common.Hash, error) {

	start := time.Now()
	tx := types.NewTransaction(w.Nonce,
		w.Account,
		big.NewInt(w.Exerciser.Amount),
		uint64(w.Exerciser.Gas),
		big.NewInt(w.Exerciser.GasPrice),
		w.CompiledContract.PackedCall)
	txHash := tx.Hash()
	w.debug("TX:%s Nonce=%d To=%s Amount=%d Gas=%d GasPrice=%d",
		txHash.Hex(), w.Nonce, tx.To().Hex(), w.Exerciser.Amount, w.Exerciser.Gas, w.Exerciser.GasPrice)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	if w.Exerciser.ExternalSign {
		err = w.sendSignedTransaction(ctx, tx)
	}
	callTime := time.Now().Sub(start)
	ok := (err == nil)
	w.info("Sent TX:%s transaction. OK=%t [%.2fs]", txHash.Hex(), ok, callTime.Seconds())

	return txHash, err
}

// waitUntilMined waits until a given transaction has been mined
func (w *Worker) waitUntilMined(txHash common.Hash) error {

	var isMined = false
	start := time.Now()

	// Transactions will not be mined immediately.
	// Wait for number of configurable number of seconds before attempting
	// to check for the transaction receipt.
	// ** This should be greater than the block period **
	time.Sleep(time.Duration(w.Exerciser.ReceiptWaitMin) * time.Second)

	// After this initial sleep we wait up to a maximum time,
	// checking periodically - 5 times up to the maximum
	retryDelay := time.Duration(float64(w.Exerciser.ReceiptWaitMax-w.Exerciser.ReceiptWaitMin)/5) * time.Second

	for !isMined {
		callStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		receipt, err := w.EthClient.TransactionReceipt(ctx, txHash)
		elapsed := time.Now().Sub(start)
		callTime := time.Now().Sub(callStart)
		isMined = receipt != nil
		w.info("Requested TX:%s receipt. Mined=%t after %.2fs [%.2fs]", txHash.Hex(), isMined, elapsed.Seconds(), callTime.Seconds())
		if err != nil && err != ethereum.NotFound {
			return fmt.Errorf("Requesting TX receipt: %s", err)
		}
		if elapsed > time.Duration(w.Exerciser.ReceiptWaitMax)*time.Second {
			return fmt.Errorf("Timed out waiting for TX receipt after %.2fs: %s", elapsed.Seconds(), err)
		}
		if !isMined {
			time.Sleep(retryDelay)
		} else {
			w.debug("TX:%s receipt: %s", receipt)
		}
	}

	return nil
}

// Init the account and connection for this worker
func (w *Worker) Init() error {

	// Generate or allocate an account from the exerciser
	if w.Exerciser.ExternalSign {
		if err := w.generateAccount(); err != nil {
			return err
		}
	} else {
		account := w.Exerciser.Accounts[w.Index]
		if !common.IsHexAddress(account) {
			return fmt.Errorf("Invalid account address (20 hex bytes with '0x' prefix): %s", account)
		}
		w.Account = common.HexToAddress(account)
	}

	// Connect the client
	rpc, err := rpc.Dial(w.Exerciser.URL)
	if err != nil {
		return fmt.Errorf("Connect to %s failed: %s", w.Exerciser.URL, err)
	}
	w.RPC = rpc
	w.EthClient = ethclient.NewClient(w.RPC)
	log.Debug(w.Name, ": connected. URL=%s", w.Exerciser.URL)
	return nil
}

// Run executes the specified exerciser workload then exits
func (w *Worker) Run() {
	log.Debug(w.Name, ": started. Account=%s", w.Account.Hex())

	infinite := (w.Exerciser.Loops == 0)
	for ; w.LoopIndex < uint64(w.Exerciser.Loops) || infinite; w.LoopIndex++ {
		txHash, err := w.sendTransaction()
		if err != nil {
			w.error("failed sending TX: %s", err)
		} else {
			err = w.waitUntilMined(txHash)
			if err != nil {
				w.error("failed checking TX receipt: %s", err)
			} else {
				w.Nonce++
			}
		}
	}

	w.RPC.Close()
	log.Debug(w.Name, ": finished. Loops=", w.LoopIndex)
}
