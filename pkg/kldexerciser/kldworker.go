package kldexerciser

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ecrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	log "github.com/sirupsen/logrus"
)

// Worker runs the specified number transactions the specified number of times then exits
type Worker struct {
	Index            int
	Name             string
	Exerciser        *Exerciser
	CompiledContract *CompiledSolidity
	Client           *rpc.Client
	Account          common.Address
	PrivateKey       *ecdsa.PrivateKey
	Signer           types.EIP155Signer
}

func (w Worker) debug(message string, inserts ...interface{}) {
	log.Debug(w.Name, ": ", fmt.Sprintf(message, inserts...))
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
	client, err := rpc.Dial(w.Exerciser.URL)
	if err != nil {
		return fmt.Errorf("Connect to %s failed: %s", w.Exerciser.URL, err)
	}
	w.Client = client
	w.debug("connected to %s", w.Exerciser.URL)
	return nil
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
func (w *Worker) signTxn(tx *types.Transaction) string {
	signedTx, _ := types.SignTx(tx, w.Signer, w.PrivateKey)
	var buff bytes.Buffer
	signedTx.EncodeRLP(&buff)
	return fmt.Sprintf("0x%x", buff.Bytes())
}

// Run executes the specified exerciser workload then exits
func (w *Worker) Run() {
	w.debug("started")

	w.Client.Close()
	w.debug("finished")
}
