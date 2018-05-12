package kldexerciser

import (
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/common/compiler"
	log "github.com/sirupsen/logrus"
)

// CompiledSolidity wraps solc compilation of solidity and ABI generation
type CompiledSolidity struct {
	SolidityFile string
	Compiled     string
	ContractInfo compiler.ContractInfo
}

// Compile uses solc to compile the Solidity source and
func (c CompiledSolidity) Compile() error {

	// Compile the solidity
	compiled, err := compiler.CompileSolidity("solc", c.SolidityFile)
	if err != nil {
		return fmt.Errorf("Solidity compilation failed: %s", err)
	}

	// Check we only have one conract and grab the code/info
	contractNames := reflect.ValueOf(compiled).MapKeys()
	if len(contractNames) != 1 {
		return fmt.Errorf("Did not find exactly one contracts in Solidity file: %s", contractNames)
	}

	contractName := contractNames[0].String()
	contract := compiled[contractName]
	c.ContractInfo = contract.Info
	c.Compiled = contract.Code
	log.Debug("Compiled contract %s", contractName)
	return nil
}
