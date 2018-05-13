package kldexerciser

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
)

// Exerciser is the Kaleido go-ethereum exerciser
type Exerciser struct {
	URL            string
	Contract       string
	Method         string
	Args           []string
	SolidityFile   string
	ABI            string
	Loops          int
	TxnsPerLoop    int
	ReceiptWaitMin int
	ReceiptWaitMax int
	Workers        int
	DebugLevel     int
	ExternalSign   bool
	ChainID        int64
	Accounts       []string
}

// Start initializes the workers for the specified config
func (e *Exerciser) Start() error {

	if e.ExternalSign && e.ChainID <= 0 {
		return fmt.Errorf("ChainID not supplied. Required for external signing")
	} else if !e.ExternalSign && len(e.Accounts) < e.Workers {
		return fmt.Errorf("Need accounts for each of %d workers (%d supplied)", e.Workers, len(e.Accounts))
	}

	log.Debug("Compiling solidity file ", e.SolidityFile)
	compiled, err := CompileContract(e.SolidityFile, e.Method, e.Args)
	if err != nil {
		return err
	}
	log.Info("Exercising method '", e.Method, "' in solidity contract ", e.SolidityFile)

	log.Debug("Starting workers. Count=", e.Workers)
	var workers = make([]Worker, e.Workers)
	var wg sync.WaitGroup
	for idx, worker := range workers {
		worker.Name = fmt.Sprintf("W%04d", idx)
		worker.Index = idx
		worker.CompiledContract = compiled
		worker.Exerciser = e
		if err := worker.Init(); err != nil {
			return err
		}
		wg.Add(1)
		go func(worker *Worker) {
			worker.Run()
			wg.Done()
		}(&worker)
	}
	wg.Wait()
	log.Debug("All workers complete")

	return nil
}
