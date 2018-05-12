package kldexerciser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/compiler"
)

// CompiledSolidity wraps solc compilation of solidity and ABI generation
type CompiledSolidity struct {
	Compiled     string
	ContractInfo compiler.ContractInfo
	PackedCall   []byte
}

// CompileContract uses solc to compile the Solidity source and
func CompileContract(solidityFile, method string, args []string) (*CompiledSolidity, error) {
	var c CompiledSolidity

	// Compile the solidity
	compiled, err := compiler.CompileSolidity("solc", solidityFile)
	if err != nil {
		return nil, fmt.Errorf("Solidity compilation failed: %s", err)
	}

	// Check we only have one conract and grab the code/info
	contractNames := reflect.ValueOf(compiled).MapKeys()
	if len(contractNames) != 1 {
		return nil, fmt.Errorf("Did not find exactly one contracts in Solidity file: %s", contractNames)
	}
	contractName := contractNames[0].String()
	contract := compiled[contractName]
	c.ContractInfo = contract.Info
	c.Compiled = contract.Code

	// Pack the arguments for calling the contract
	abiJSON, err := json.Marshal(contract.Info.AbiDefinition)
	if err != nil {
		return nil, fmt.Errorf("Serializing ABI: %s", err)
	}
	abi, err := abi.JSON(bytes.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("Parsing ABI: %s", err)
	}
	packedCall, err := abi.Pack(method, args)
	if err != nil {
		return nil, fmt.Errorf("Packing arguments %s for call %s: %s", args, method, err)
	}
	c.PackedCall = packedCall

	return &c, nil
}
