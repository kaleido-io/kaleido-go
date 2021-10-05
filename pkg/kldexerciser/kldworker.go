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
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/avast/retry-go"
	hexutil "github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/ethereum/go-ethereum"
	ecrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	log "github.com/sirupsen/logrus"
)

// Worker runs the specified number transactions the specified number of times then exits
type Worker struct {
	Index                 int
	Name                  string
	LoopIndex             uint64
	Exerciser             *Exerciser
	Nonce                 uint64
	CompiledContract      *CompiledSolidity
	RPC                   *rpc.Client
	Account               common.Address
	PrivateKey            *ecdsa.PrivateKey
	Signer                types.EIP155Signer
	servername            string
	pid                   int
	telegrafMetricsFormat bool
	metricsQualifier      string
	lastMiningTime        time.Duration
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

	var err error
	var txHash string
	if w.Exerciser.ExternalSign {
		txHash, err = w.signAndSendTxn(tx)
	} else {
		txHash, err = w.sendUnsignedTxn(tx)
	}
	callTime := time.Since(start)
	ok := (err == nil)

	w.incrCounter("tx.sub")
	if ok {
		w.incrCounter("tx.sent")
	} else {
		w.incrCounter("tx.sendfail")
		w.incrCounter("tx.fail")
	}

	if !ok && (strings.Contains(err.Error(), "known transaction") || strings.Contains(err.Error(), "nonce too low")) {
		// Bump the nonce for the next attempt
		w.Nonce++
	}

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
	// EEA spec extensions
	PrivateFrom string   `json:"privateFrom,omitempty"`
	PrivateFor  []string `json:"privateFor,omitempty"`
}

// sendUnsignedTxn sends a transaction for internal signing by the node
func (w *Worker) sendUnsignedTxn(tx *types.Transaction) (string, error) {
	data := hexutil.Bytes(tx.Data())
	args := sendTxArgs{
		Nonce:    hexutil.Uint64(w.Nonce),
		From:     w.Account.Hex(),
		Gas:      hexutil.Uint64(tx.Gas()),
		GasPrice: hexutil.Big(*tx.GasPrice()),
		Value:    hexutil.Big(*tx.Value()),
		Data:     &data,
	}
	if w.Exerciser.PrivateFrom != "" {
		args.PrivateFrom = w.Exerciser.PrivateFrom
		args.PrivateFor = w.Exerciser.PrivateFor
	}
	var to = tx.To()
	if to != nil {
		args.To = to.Hex()
	}
	var txHash string
	err := w.rpcCall(&txHash, "eth_sendTransaction", args)
	return txHash, err
}

// Formatter for Telegraf/InfluxDB style statsd metrics formatting
func (w *Worker) telegrafMetricsFormatter(stat string) string {
	qual := ""
	if w.metricsQualifier != "" {
		qual = ",qual=" + w.metricsQualifier
	}
	return fmt.Sprintf("%s,server=%s,pid=%d,worker=%d%s", stat, w.servername, w.pid, w.Index, qual)
}

// Formatter for Graphite style statsd metrics formatting
func (w *Worker) graphiteMetricsFormatter(stat string) string {
	qual := ""
	if w.metricsQualifier != "" {
		qual = w.metricsQualifier + "."
	}
	return fmt.Sprintf("%s%s.P%06d%s.%s", qual, w.servername, w.pid, w.Name, stat)
}

func (w *Worker) incrCounter(name string) {
	if w.Exerciser.metrics != nil {
		if w.telegrafMetricsFormat {
			w.Exerciser.metrics.Increment(w.telegrafMetricsFormatter(name))
		} else {
			w.Exerciser.metrics.Increment(w.graphiteMetricsFormatter(name))
		}
	}
}

func (w *Worker) emitTiming(name string, timing time.Duration) {
	if w.Exerciser.metrics != nil {
		if w.telegrafMetricsFormat {
			w.Exerciser.metrics.Gauge(w.telegrafMetricsFormatter(name), int(timing.Nanoseconds()/1000000))
		} else {
			w.Exerciser.metrics.Gauge(w.graphiteMetricsFormatter(name), int(timing.Nanoseconds()/1000000))
		}
	}
}

func (w *Worker) rpcCall(result interface{}, method string, args ...interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(w.Exerciser.RPCTimeout)*time.Second)
	defer cancel()

	err := retry.Do(
		func() error {
			log.Debugf("Invoking %s", method)
			w.incrCounter("rpc.calls")
			err := w.RPC.CallContext(ctx, result, method, args...)
			if err == nil {
				w.incrCounter("rpc.success")
			} else {
				w.incrCounter("rpc.fail")
			}
			return err
		},
		retry.RetryIf(func(err error) bool {
			return strings.Contains(err.Error(), "429")
		}),
		retry.Attempts(10),
		retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
			log.Debugf("%s attempt %d failed: %s", method, n, err)
			return retry.BackOffDelay(n, err, config)
		}),
	)
	return err
}

// callContract call a transaction and return the result as a string
func (w *Worker) callContract(tx *types.Transaction) (err error) {

	start := time.Now()

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

	if w.Exerciser.EstimateGas {
		var retValue hexutil.Uint64
		err = w.rpcCall(&retValue, "eth_estimateGas", args)
		callTime := time.Since(start)
		w.info("estimate Gas result: %d [%.2fs]", retValue, callTime.Seconds())
	} else {
		var retValue string
		err = w.rpcCall(&retValue, "eth_call", args, "latest")
		callTime := time.Since(start)
		w.info("call result: '%s' [%.2fs]", retValue, callTime.Seconds())
	}
	if err != nil {
		return fmt.Errorf("contract call failed: %s", err)
	}

	return
}

// signAndSendTxn externally signs and sends a transaction
func (w *Worker) signAndSendTxn(tx *types.Transaction) (string, error) {
	signedTx, _ := types.SignTx(tx, w.Signer, w.PrivateKey)
	var buff bytes.Buffer
	signedTx.EncodeRLP(&buff)
	from, _ := types.Sender(w.Signer, signedTx)
	w.debug("TX signed. ChainID=%d From=%s", w.Exerciser.ChainID, from.Hex())
	if from.Hex() != w.Account.Hex() {
		return "", fmt.Errorf("EIP155 signing failed - Account=%s From=%s", w.Account.Hex(), from.Hex())
	}

	var txHash string
	data, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return txHash, fmt.Errorf("failed to RLP encode: %s", err)
	}
	err = w.rpcCall(&txHash, "eth_sendRawTransaction", "0x"+hex.EncodeToString(data))
	return txHash, err
}

type txnReceipt struct {
	BlockHash         *common.Hash    `json:"blockHash"`
	BlockNumber       *hexutil.Big    `json:"blockNumber"`
	ContractAddress   *common.Address `json:"contractAddress"`
	CumulativeGasUsed *hexutil.Big    `json:"cumulativeGasUsed"`
	TransactionHash   *common.Hash    `json:"transactionHash"`
	From              *common.Address `json:"from"`
	GasUsed           *hexutil.Big    `json:"gasUsed"`
	Status            *hexutil.Big    `json:"status"`
	To                *common.Address `json:"to"`
	TransactionIndex  *hexutil.Uint   `json:"transactionIndex"`
}

// WaitUntilMined waits until a given transaction has been mined
func (w *Worker) waitUntilMined(start time.Time, txHash string, retryDelay time.Duration) (*txnReceipt, error) {

	var isMined = false

	var receipt txnReceipt
	for !isMined {
		callStart := time.Now()

		err := w.rpcCall(&receipt, "eth_getTransactionReceipt", common.HexToHash(txHash))
		elapsed := time.Since(start)
		callTime := time.Since(callStart)

		isMined = receipt.BlockNumber != nil && receipt.BlockNumber.ToInt().Uint64() > 0
		w.info("TX:%s Mined=%t after %.2fs [%.2fs]", txHash, isMined, elapsed.Seconds(), callTime.Seconds())
		if err != nil && err != ethereum.NotFound {
			return nil, fmt.Errorf("requesting TX receipt: %s", err)
		}
		if receipt.Status != nil {
			status := receipt.Status.ToInt()
			w.incrCounter("tx.receipt")
			if status.Uint64() == 1 {
				w.incrCounter("tx.success")
			} else {
				w.incrCounter("tx.failexec")
				w.incrCounter("tx.fail")
			}

			w.debug("Status=%s BlockNumber=%s BlockHash=%x TransactionIndex=%d GasUsed=%s CumulativeGasUsed=%s",
				status, receipt.BlockNumber.ToInt(), receipt.BlockHash,
				receipt.TransactionIndex, receipt.GasUsed.ToInt(), receipt.CumulativeGasUsed.ToInt())
		}
		if !isMined && elapsed > time.Duration(w.Exerciser.ReceiptWaitMax)*time.Second {
			w.incrCounter("tx.timeout")
			w.incrCounter("tx.fail")
			return nil, fmt.Errorf("timed out waiting for TX receipt after %.2fs", elapsed.Seconds())
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
		// Wait for mining
		start := time.Now()
		w.debug("Waiting for %d seconds for tx be mined in next block", w.Exerciser.ReceiptWaitMin)
		time.Sleep(time.Duration(w.Exerciser.ReceiptWaitMin) * time.Second)
		receipt, err = w.waitUntilMined(start, txHash, 1*time.Second)
		if err != nil {
			return nil, fmt.Errorf("failed checking TX receipt: %s", err)
		}
		// Increase nonce only if we got a receipt.
		// Known transaction processing will kick in to bump the nonce otherwise
		w.Nonce++
	}
	return receipt, err
}

// initializeNonce get the initial nonce to use
func (w *Worker) initializeNonce(address common.Address) error {
	block := "latest"
	if w.Exerciser.Nonce == -1 {
		var result hexutil.Uint64
		err := w.RPC.Call(&result, "eth_getTransactionCount", address.Hex(), block)
		if err != nil {
			return fmt.Errorf("failed to get transaction count '%s' for %s: %s", result, address, err)
		}
		w.debug("Received nonce=%d for %s at '%s' block", result, address.Hex(), block)
		w.Nonce = uint64(result)
	} else {
		w.Nonce = uint64(w.Exerciser.Nonce)
	}
	return nil
}

// Init the account and connection for this worker
func (w *Worker) Init(rpc *rpc.Client) (err error) {
	w.RPC = rpc

	// Store items we need for metrics naming
	if w.Exerciser.StatsdServer != "" {
		hostname, _ := os.Hostname()
		w.servername = strings.Split(hostname, ".")[0]
		w.pid = os.Getpid()
		w.telegrafMetricsFormat = w.Exerciser.StatsdTelegraf
		w.metricsQualifier = w.Exerciser.StatsdQualifier
	}

	// Generate or allocate an account from the exerciser
	if w.Exerciser.ExternalSign {
		w.Account = ecrypto.PubkeyToAddress(w.PrivateKey.PublicKey)
		w.Signer = types.NewEIP155Signer(big.NewInt(w.Exerciser.ChainID))
	} else {
		account := w.Exerciser.Accounts[w.Index]
		if !common.IsHexAddress(account) {
			return fmt.Errorf("invalid account address (20 hex bytes with '0x' prefix): %s", account)
		}
		w.Account = common.HexToAddress(account)
	}

	// Get the initial nonce for this existing account
	if err := w.initializeNonce(w.Account); err != nil {
		return err
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
		return nil, fmt.Errorf("failed to install contract: %s", err)
	}
	return receipt.ContractAddress, nil
}

// CallOnce executes a contract once and returns
func (w *Worker) CallOnce() error {
	tx := w.generateTransaction()
	err := w.callContract(tx)
	return err
}

// CallMultiple executes a contract based on loop inputs
func (w *Worker) CallMultiple() {
	infinite := (w.Exerciser.Loops == 0)
	for ; w.LoopIndex < uint64(w.Exerciser.Loops) || infinite; w.LoopIndex++ {

		tx := w.generateTransaction()
		_ = w.callContract(tx)
		time.Sleep(time.Duration(w.Exerciser.ReceiptWaitMin) * time.Second)
	}
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
				txHashes = append(txHashes, txHash)
				w.Nonce++
			}
		}

		// Find the difference between the last successful mining time, and the initial wait
		// period. Aim for two retries per round, by roughly spliting the difference between
		// the minimum time, and 1 second over the last mining time.
		minSleep := time.Duration(w.Exerciser.ReceiptWaitMin) * time.Second
		if w.lastMiningTime < minSleep {
			w.lastMiningTime = minSleep
		}
		retryDelay := (w.lastMiningTime - minSleep + 1*time.Second) / 2
		if retryDelay < 1*time.Second {
			retryDelay = 1 * time.Second
		}
		// Reset the lastMiningTime each time round the loop
		w.lastMiningTime = 0

		// Transactions will not be mined immediately.
		// Wait for number of configurable number of seconds before attempting
		// to check for the transaction receipt.
		// ** This should be greater than the block period **
		initialSleep := minSleep + retryDelay
		w.debug("Waiting for %.2fs seconds then retrying every %.2fs", initialSleep.Seconds(), retryDelay.Seconds())
		start := time.Now()
		time.Sleep(minSleep)

		// Wait for the receipts of all successfully set transctions
		var loopSuccesses uint64
		for _, txHash := range txHashes {
			receipt, err := w.waitUntilMined(start, txHash, retryDelay)
			if err != nil {
				w.error("TX:%s failed checking receipt: %s", txHash, err)
			} else {
				// Store the mining time for the first successful transaction
				if w.lastMiningTime == 0 {
					w.lastMiningTime = time.Since(start)
					w.emitTiming("tx.minetime", w.lastMiningTime)
					w.debug("First TX for this loop iteration mined after %.2fs", w.lastMiningTime.Seconds())
				}

				if receipt.Status.ToInt().Uint64() == 0 {
					w.error("TX:%s failed. Status=%s", txHash, receipt.Status.ToInt())
					// If gasUsed == gasProvided, then you ran out of gas
					if receipt.GasUsed.ToInt().Uint64() == uint64(w.Exerciser.Gas) {
						w.error("TX ran out of gas before completion.")
					}
				} else {
					loopSuccesses++
				}
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
