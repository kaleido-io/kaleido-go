// Copyright 2018 Kaleido, a ConsenSys business

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kldexerciser

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum/go-ethereum"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ecrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
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
	Account          common.Address
	PrivateKey       *ecdsa.PrivateKey
	Signer           types.EIP155Signer
}

func (w Worker) debug(message string, inserts ...interface{}) {
	log.Debug(fmt.Sprintf("%s/L%04d/N%06d: ", w.Name, w.LoopIndex, w.Nonce), fmt.Sprintf(message, inserts...))
}

func (w Worker) info(message string, inserts ...interface{}) {
	log.Info(fmt.Sprintf("%s/L%04d/N%06d: ", w.Name, w.LoopIndex, w.Nonce), fmt.Sprintf(message, inserts...))
}

func (w Worker) error(message string, inserts ...interface{}) {
	log.Error(fmt.Sprintf("%s/L%04d/N%06d: ", w.Name, w.LoopIndex, w.Nonce), fmt.Sprintf(message, inserts...))
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

// generateTransaction creates a new transaction for the specified data
func (w *Worker) generateTransaction() *types.Transaction {
	tx := types.NewTransaction(
		w.Nonce,
		*w.Exerciser.To,
		big.NewInt(w.Exerciser.Amount),
		uint64(w.Exerciser.Gas),
		big.NewInt(w.Exerciser.GasPrice),
		w.CompiledContract.PackedCall)
	w.debug("TX:%s To=%s Amount=%d Gas=%d GasPrice=%d",
		tx.Hash().Hex(), tx.To().Hex(), w.Exerciser.Amount, w.Exerciser.Gas, w.Exerciser.GasPrice)
	return tx
}

// sendTransaction sends an individual transaction, choosing external or internal signing
func (w *Worker) sendTransaction(tx *types.Transaction) (string, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	var txHash string
	if w.Exerciser.ExternalSign {
		txHash, err = w.signAndSendTxn(ctx, tx)
	} else {
		txHash, err = w.sendUnsignedTxn(ctx, tx)
	}
	callTime := time.Now().Sub(start)
	ok := (err == nil)
	w.info("TX:%s Sent. OK=%t [%.2fs]", txHash, ok, callTime.Seconds())
	return txHash, err
}

type sendTxArgs struct {
	Nonce    hexutil.Uint64 `json:"nonce"`
	From     string         `json:"from"`
	To       string         `json:"to,omitempty"`
	Gas      hexutil.Uint64 `json:"gas"`
	GasPrice hexutil.Big    `json:"gasPrice"`
	Value    hexutil.Big    `json:"value"`
	Data     *hexutil.Bytes `json:"data"`
}

// sendUnsignedTxn sends a transaction for internal signing by the node
func (w *Worker) sendUnsignedTxn(ctx context.Context, tx *types.Transaction) (string, error) {
	data := hexutil.Bytes(tx.Data())
	args := sendTxArgs{
		Nonce:    hexutil.Uint64(w.Nonce),
		From:     w.Account.Hex(),
		Gas:      hexutil.Uint64(tx.Gas()),
		GasPrice: hexutil.Big(*tx.GasPrice()),
		Value:    hexutil.Big(*tx.Value()),
		Data:     &data,
	}
	var to = tx.To()
	if to != nil {
		args.To = to.Hex()
	}
	var txHash string
	err := w.RPC.CallContext(ctx, &txHash, "eth_sendTransaction", args)
	return txHash, err
}

// callContract call a transaction and return the result as a string
func (w *Worker) callContract(tx *types.Transaction) (string, error) {

	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data := hexutil.Bytes(tx.Data())
	args := sendTxArgs{
		Nonce:    hexutil.Uint64(w.Nonce),
		From:     w.Account.Hex(),
		To:       tx.To().Hex(),
		Gas:      hexutil.Uint64(tx.Gas()),
		GasPrice: hexutil.Big(*tx.GasPrice()),
		Value:    hexutil.Big(*tx.Value()),
		Data:     &data,
	}
	var retValue string
	err := w.RPC.CallContext(ctx, &retValue, "eth_call", args, "latest")
	if err != nil {
		return "", fmt.Errorf("Contract call failed: %s", err)
	}
	callTime := time.Now().Sub(start)
	w.info("Call result: '%s' [%.2fs]", retValue, callTime.Seconds())
	return retValue, nil
}

// signAndSendTxn externally signs and sends a transaction
func (w *Worker) signAndSendTxn(ctx context.Context, tx *types.Transaction) (string, error) {
	signedTx, _ := types.SignTx(tx, w.Signer, w.PrivateKey)
	var buff bytes.Buffer
	signedTx.EncodeRLP(&buff)
	from, _ := types.Sender(w.Signer, signedTx)
	w.debug("TX signed. ChainID=%d From=%s", w.Exerciser.ChainID, from.Hex())

	var txHash string
	data, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return txHash, fmt.Errorf("Failed to RLP encode: %s", err)
	}
	err = w.RPC.CallContext(ctx, &txHash, "eth_sendRawTransaction", common.ToHex(data))
	return txHash, err
}

type txnReceipt struct {
	BlockHash         string `json:"blockHash"`
	BlockNumber       string `json:"blockNumber"`
	ContractAddress   string `json:"contractAddress"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	TransactionHash   string `json:"transactionHash"`
	From              string `json:"from"`
	GasUsed           string `json:"gasUsed"`
	Status            string `json:"status"`
	To                string `json:"to"`
	TransactionIndex  string `json:"transactionIndex"`
}

// WaitUntilMined waits until a given transaction has been mined
func (w *Worker) waitUntilMined(start time.Time, txHash string) (*txnReceipt, error) {

	var isMined = false

	// After this initial sleep we wait up to a maximum time,
	// checking periodically - 5 times up to the maximum
	retryDelay := time.Duration(float64(w.Exerciser.ReceiptWaitMax-w.Exerciser.ReceiptWaitMin)/5) * time.Second

	var receipt txnReceipt
	for !isMined {
		callStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := w.RPC.CallContext(ctx, &receipt, "eth_getTransactionReceipt", common.HexToHash(txHash))
		elapsed := time.Now().Sub(start)
		callTime := time.Now().Sub(callStart)

		isMined = receipt.Status == "0x1"
		w.info("TX:%s Mined=%t after %.2fs [%.2fs]", txHash, isMined, elapsed.Seconds(), callTime.Seconds())
		if err != nil && err != ethereum.NotFound {
			return nil, fmt.Errorf("Requesting TX receipt: %s", err)
		}
		w.debug("Status=%s BlockNumber=%s BlockHash=%s TransactionIndex=%s GasUsed=%s CumulativeGasUsed=%s",
			receipt.Status, receipt.BlockNumber, receipt.BlockHash, receipt.TransactionIndex, receipt.GasUsed, receipt.CumulativeGasUsed)
		if !isMined && elapsed > time.Duration(w.Exerciser.ReceiptWaitMax)*time.Second {
			return nil, fmt.Errorf("Timed out waiting for TX receipt after %.2fs: %s", elapsed.Seconds(), err)
		}
		if !isMined {
			time.Sleep(retryDelay)
		}
	}

	return &receipt, nil
}

// SendAndWaitForMining sends a single transaction and waits for it to be mined
func (w *Worker) sendAndWaitForMining(tx *types.Transaction) (*txnReceipt, error) {
	txHash, err := w.sendTransaction(tx)
	var receipt *txnReceipt
	if err != nil {
		w.error("failed sending TX: %s", err)
	} else {
		w.Nonce++
		// Wait for mining
		start := time.Now()
		w.debug("Waiting for %d seconds for tx be mined in next block", w.Exerciser.ReceiptWaitMin)
		time.Sleep(time.Duration(w.Exerciser.ReceiptWaitMin) * time.Second)
		receipt, err = w.waitUntilMined(start, txHash)
		if err != nil {
			return nil, fmt.Errorf("failed checking TX receipt: %s", err)
		}
	}
	return receipt, err
}

// initializeNonce get the initial nonce to use
func (w *Worker) initializeNonce(address string) error {
	var result hexutil.Uint64
	err := w.RPC.Call(&result, "eth_getTransactionCount", address, "latest")
	if err != nil {
		return fmt.Errorf("Failed to get transaction count for %s: %s", address, err)
	}
	w.Nonce = uint64(result)
	if err != nil {
		return fmt.Errorf("Failed to parse transaction count '%s' for %s: %s", result, address, err)
	}
	return nil
}

// Init the account and connection for this worker
func (w *Worker) Init() error {

	// Connect the client
	rpc, err := rpc.Dial(w.Exerciser.URL)
	if err != nil {
		return fmt.Errorf("Connect to %s failed: %s", w.Exerciser.URL, err)
	}
	w.RPC = rpc
	log.Debug(w.Name, ": connected. URL=", w.Exerciser.URL)

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

		// Get the initial nonce for this existing account
		if err := w.initializeNonce(account); err != nil {
			return err
		}
	}

	return nil
}

// InstallContract installs the contract and returns the address
func (w *Worker) InstallContract() (*common.Address, error) {
	tx := types.NewContractCreation(
		w.Nonce,
		big.NewInt(w.Exerciser.Amount),
		uint64(w.Exerciser.Gas),
		big.NewInt(w.Exerciser.GasPrice),
		common.FromHex(w.CompiledContract.Compiled),
	)
	receipt, err := w.sendAndWaitForMining(tx)
	if err != nil {
		return nil, fmt.Errorf("Failed to install contract: %s", err)
	}
	contractAddr := common.HexToAddress(receipt.ContractAddress)
	return &contractAddr, nil
}

// Call executes a contract once and returns
func (w *Worker) CallOnce() error {
	tx := w.generateTransaction()
	_, err := w.callContract(tx)
	return err
}

// Run executes the specified exerciser workload then exits
func (w *Worker) Run() {
	log.Debug(w.Name, ": started. ", w.Exerciser.TxnsPerLoop, " tx/loop for ", w.Exerciser.Loops, " loops. Account=", w.Account.Hex())

	var successes, failures uint64
	infinite := (w.Exerciser.Loops == 0)
	for ; w.LoopIndex < uint64(w.Exerciser.Loops) || infinite; w.LoopIndex++ {

		// Send a set of transactions before waiting for receipts (which takes some time)
		var txHashes []string
		for i := 0; i < w.Exerciser.TxnsPerLoop; i++ {
			tx := w.generateTransaction()
			txHash, err := w.sendTransaction(tx)
			if err != nil {
				w.error("TX send failed (%d/%d): %s", i, w.Exerciser.TxnsPerLoop, err)
			} else {
				w.Nonce++
				txHashes = append(txHashes, txHash)
			}
		}

		// Transactions will not be mined immediately.
		// Wait for number of configurable number of seconds before attempting
		// to check for the transaction receipt.
		// ** This should be greater than the block period **
		w.debug("Waiting for %d seconds for tx be mined in next block", w.Exerciser.ReceiptWaitMin)
		start := time.Now()
		time.Sleep(time.Duration(w.Exerciser.ReceiptWaitMin) * time.Second)

		// Wait the receipts of all successfully set transctions
		var loopSuccesses uint64
		for _, txHash := range txHashes {
			_, err := w.waitUntilMined(start, txHash)
			if err != nil {
				w.error("TX:%s failed checking receipt: %s", err)
			} else {
				loopSuccesses++
			}
		}
		var loopFailures = uint64(w.Exerciser.TxnsPerLoop) - loopSuccesses

		// Increment global counters
		successes += loopSuccesses
		atomic.AddUint64(&w.Exerciser.TotalSuccesses, loopSuccesses)
		failures += loopFailures
		atomic.AddUint64(&w.Exerciser.TotalFailures, loopFailures)
	}

	w.RPC.Close()
	log.Debug(w.Name, ": finished. Loops=", w.LoopIndex, " Success=", successes, " Failures=", failures)
}
