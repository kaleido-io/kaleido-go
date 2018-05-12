package kldexerciser

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

// Exerciser is the Kaleido go-ethereum exerciser
type Exerciser struct {
	URL          string
	Contract     string
	SolidityFile string
	ABI          string
	Txns         int
	Workers      int
	DebugLevel   int
	ExternalSign bool
	ChainID      uint64
	Accounts     []string
}

// Start initializes the workers for the specified config
func (e Exerciser) Start() error {

	if !e.ExternalSign && len(e.Accounts) < e.Workers {
		return fmt.Errorf("Need accounts for each of %d workers (%d supplied)", e.Workers, len(e.Accounts))
	}

	log.Debug("Compiling solidity file ", e.SolidityFile)
	compiled := CompiledSolidity{SolidityFile: e.SolidityFile}
	if err := compiled.Compile(); err != nil {
		return err
	}

	return nil
}
