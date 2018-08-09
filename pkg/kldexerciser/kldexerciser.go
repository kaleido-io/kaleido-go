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
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"

	log "github.com/sirupsen/logrus"
)

// Exerciser is the Kaleido go-ethereum exerciser
type Exerciser struct {
	URL            string
	Call           bool
	EstimateGas    bool
	Contract       string
	To             *common.Address
	ContractName   string
	Method         string
	Args           []string
	SolidityFile   string
	ABI            string
	Amount         int64
	GasPrice       int64
	Gas            int64
	PrivateFrom    string
	PrivateFor     []string
	Loops          int
	TxnsPerLoop    int
	ReceiptWaitMin int
	ReceiptWaitMax int
	Workers        int
	DebugLevel     int
	ExternalSign   bool
	ChainID        int64
	Accounts       []string
	TotalSuccesses uint64
	TotalFailures  uint64
	Nonce          int64
}

// Start initializes the workers for the specified config
func (e *Exerciser) Start() error {

	if !e.ExternalSign && len(e.Accounts) < e.Workers {
		return fmt.Errorf("Need accounts for each of %d workers (%d supplied)", e.Workers, len(e.Accounts))
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

	log.Debug("Connecting workers. Count=", e.Workers)
	var workers = make([]Worker, e.Workers)
	for i := 0; i < len(workers); i++ {
		worker := &workers[i]
		worker.Name = fmt.Sprintf("W%04d", i)
		worker.Index = i
		worker.CompiledContract = compiled
		worker.Exerciser = e
		if err := worker.Init(); err != nil {
			return err
		}
	}

	firstWorker := &workers[0]

	if e.ExternalSign && e.ChainID <= 0 {
		if e.ChainID, err = firstWorker.GetNetworkID(); err != nil {
			return err
		}
		log.Debug("ChainID=", e.ChainID)
	}

	if e.Contract == "" {
		log.Info("Deploying contract using worker ", firstWorker.Name)
		e.To, err = firstWorker.InstallContract()
		if err != nil {
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
