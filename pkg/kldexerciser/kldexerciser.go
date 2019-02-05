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
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ecrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/alexcesaro/statsd"
	log "github.com/sirupsen/logrus"
)

// Exerciser is the Kaleido go-ethereum exerciser
type Exerciser struct {
	URL               string
	Call              bool
	EstimateGas       bool
	Contract          string
	To                *common.Address
	ContractName      string
	Method            string
	Args              []string
	SolidityFile      string
	ABI               string
	Amount            int64
	GasPrice          int64
	Gas               int64
	StatsdServer      string
	StatsdFlushPeriod int64
	StatsdTelegraf    bool
	RPCTimeout        int
	PrivateFrom       string
	PrivateFor        []string
	Loops             int
	TxnsPerLoop       int
	ReceiptWaitMin    int
	ReceiptWaitMax    int
	Workers           int
	DebugLevel        int
	ExternalSign      bool
	ExternalSignJSON  string
	ChainID           int64
	Accounts          []string
	TotalSuccesses    uint64
	TotalFailures     uint64
	Nonce             int64
	metrics           *statsd.Client
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (e *Exerciser) ensurePrivateKeys() (keys []*ecdsa.PrivateKey, err error) {
	preGenerated := 0

	// Load existing keys
	if e.ExternalSignJSON != "" {
		jsonData, err := ioutil.ReadFile(e.ExternalSignJSON)
		var serializedKeys = []string{}
		if err != nil {
			log.Infof("File %s does not exist, it will be created (%s)", e.ExternalSignJSON, err)
		} else if err = json.Unmarshal(jsonData, &serializedKeys); err != nil {
			return nil, fmt.Errorf("Unable to parse %s: %s", e.ExternalSignJSON, err)
		}
		preGenerated = len(serializedKeys)
		keys = make([]*ecdsa.PrivateKey, max(preGenerated, e.Workers))
		for i := 0; i < preGenerated; i++ {
			if keys[i], err = ecrypto.HexToECDSA(serializedKeys[i]); err != nil {
				return nil, fmt.Errorf("Unable to convert hex string %d in %s to ECDSA curve: %s", i, e.ExternalSignJSON, err)
			}
		}
	} else {
		keys = make([]*ecdsa.PrivateKey, e.Workers)
	}

	// Generate any missing keys
	for i := preGenerated; i < e.Workers; i++ {
		if keys[i], err = e.generateAccount(); err != nil {
			return
		}
	}

	// Serialize the updated structure if we generated some
	if e.ExternalSignJSON != "" && preGenerated < e.Workers {
		serializedKeys := make([]string, len(keys))
		for i := 0; i < len(keys); i++ {
			serializedKeys[i] = hex.EncodeToString(ecrypto.FromECDSA(keys[i]))
		}
		jsonBytes, _ := json.MarshalIndent(serializedKeys, "", "")
		err := ioutil.WriteFile(e.ExternalSignJSON, jsonBytes, 0777)
		if err != nil {
			return nil, fmt.Errorf("Unable to write %s: %s", e.ExternalSignJSON, err)
		}
	}

	return
}

// generateAccount generates a new account for this worker to use in external signing
func (e *Exerciser) generateAccount() (*ecdsa.PrivateKey, error) {
	key, err := ecrypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("Generating key: %s", err)
	}
	return key, nil
}

// GetNetworkID returns the network ID from the node
func (e *Exerciser) GetNetworkID() (int64, error) {
	rpc, err := rpc.Dial(e.URL)
	if err != nil {
		return 0, fmt.Errorf("Connect to %s failed: %s", e.URL, err)
	}
	defer rpc.Close()
	var strNetworkID string
	err = rpc.Call(&strNetworkID, "net_version")
	if err != nil {
		return 0, fmt.Errorf("Failed to query network ID (to use as chain ID in EIP155 signing): %s", err)
	}
	networkID, err := strconv.ParseInt(strNetworkID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse network ID returned from node '%s': %s", strNetworkID, err)
	}
	return networkID, nil
}

// Start initializes the workers for the specified config
func (e *Exerciser) Start() (err error) {

	if !e.ExternalSign && len(e.Accounts) < e.Workers {
		return fmt.Errorf("Need accounts for each of %d workers (%d supplied)", e.Workers, len(e.Accounts))
	}

	var keys []*ecdsa.PrivateKey
	if e.ExternalSign {
		if keys, err = e.ensurePrivateKeys(); err != nil {
			return err
		}
	}

	if e.StatsdServer != "" {
		if e.metrics, err = statsd.New(
			statsd.Address(e.StatsdServer),
			statsd.FlushPeriod(time.Duration(e.StatsdFlushPeriod)*time.Millisecond)); err != nil {
			return fmt.Errorf("Failed to create metrics sink to statsd %s: %s", e.StatsdServer, err)
		}
	}

	log.Debug("Compiling solidity file ", e.SolidityFile)
	compiled, err := CompileContract(e.SolidityFile, e.ContractName, e.Method, e.Args)
	if err != nil {
		return err
	}
	log.Info("Exercising method '", e.Method, "' in solidity contract ", e.SolidityFile)

	if e.PrivateFrom != "" {
		if e.ExternalSign {
			return fmt.Errorf("External signing not currently supported with private transactions")
		}
		log.Debug("PrivateFrom='", e.PrivateFrom, "' PrivateFor='", e.PrivateFor, "'")
	}

	if e.ExternalSign && e.ChainID <= 0 {
		if e.ChainID, err = e.GetNetworkID(); err != nil {
			return err
		}
		log.Debug("ChainID=", e.ChainID)
	}

	log.Debug("Connecting workers. Count=", e.Workers)
	var workers = make([]Worker, e.Workers)
	for i := 0; i < len(workers); i++ {
		worker := &workers[i]
		worker.Name = fmt.Sprintf("W%04d", i)
		worker.Index = i
		worker.CompiledContract = compiled
		worker.Exerciser = e
		if e.ExternalSign {
			worker.PrivateKey = keys[i]
		}
		if err := worker.Init(); err != nil {
			return err
		}
	}

	if e.Contract == "" {
		var deployed = false
		for i := 0; i < e.Workers && !deployed; i++ {
			contractWorker := &workers[i]

			log.Infof("Deploying contract using worker %s", contractWorker.Name)
			e.To, err = contractWorker.InstallContract()
			if err == nil {
				deployed = true
			} else {
				log.Errorf("Failed to deploy contract using work %s: %s", contractWorker.Name, err)
			}
		}
		if !deployed {
			return err
		}
	} else {
		if !common.IsHexAddress(e.Contract) {
			return fmt.Errorf("Invalid contract address: %s", e.Contract)
		}
		contractAddr := common.HexToAddress(e.Contract)
		e.To = &contractAddr
	}
	log.Info("Contract address=", e.To.Hex())

	if e.Call || e.EstimateGas {
		log.Debug("Calling contract")
		if err := workers[0].CallOnce(); err != nil {
			return err
		}
	} else {
		log.Debug("Starting workers. Count=", e.Workers)
		var wg sync.WaitGroup
		for i := 0; i < len(workers); i++ {
			worker := &workers[i]
			wg.Add(1)
			go func(worker *Worker) {
				worker.Run()
				wg.Done()
			}(worker)
		}
		wg.Wait()
		log.Info("All workers complete. Success=", e.TotalSuccesses, " Failure=", e.TotalFailures)
	}
	return nil
}
