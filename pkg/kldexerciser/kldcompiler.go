package kldexerciser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/compiler"
	log "github.com/sirupsen/logrus"
)

// CompiledSolidity wraps solc compilation of solidity and ABI generation
type CompiledSolidity struct {
	Compiled     string
	ContractInfo compiler.ContractInfo
	PackedCall   []byte
}

// GenerateTypedArgs parses string arguments into a range of types to pass to the ABI call
func GenerateTypedArgs(abi abi.ABI, methodName string, strargs []string) ([]interface{}, error) {

	method, exist := abi.Methods[methodName]
	if !exist {
		return nil, fmt.Errorf("Method '%s' not found", methodName)
	}

	log.Debug("Parsing args for method: ", method)
	var typedArgs []interface{}
	for idx, inputArg := range method.Inputs {
		if idx >= len(strargs) {
			return nil, fmt.Errorf("Method requires %d args: %s", len(method.Inputs), method)
		}
		strval := strargs[idx]
		switch inputArg.Type.String() {
		case "string":
			typedArgs = append(typedArgs, strval)
			break
		case "int256", "uint256":
			bigInt := big.NewInt(0)
			if _, ok := bigInt.SetString(strval, 10); !ok {
				return nil, fmt.Errorf("Could not convert '%s' to %s", strval, inputArg.Type)
			}
			typedArgs = append(typedArgs, bigInt)
			break
		case "bool":
			typedArgs = append(typedArgs, strings.ToLower(strval) == "true")
			break
		case "address":
			if !common.IsHexAddress(strval) {
				return nil, fmt.Errorf("Invalid hex address for arg %d: %s", idx, strval)
			}
			typedArgs = append(typedArgs, common.HexToAddress(strval))
			break
		default:
			return nil, fmt.Errorf("No string parsing configured yet for type %s", inputArg.Type)
		}
	}

	return typedArgs, nil

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
	typedArgs, err := GenerateTypedArgs(abi, method, args)
	if err != nil {
		return nil, err
	}
	packedCall, err := abi.Pack(method, typedArgs...)
	if err != nil {
		return nil, fmt.Errorf("Packing arguments %s for call %s: %s", args, method, err)
	}
	c.PackedCall = packedCall

	return &c, nil
}
