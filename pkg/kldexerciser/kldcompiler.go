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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/compiler"
	log "github.com/sirupsen/logrus"
)

// CompiledSolidity wraps solc compilation of solidity and ABI generation
type CompiledSolidity struct {
	Compiled     string
	ContractInfo compiler.ContractInfo
	method       string
	abi          abi.ABI
	args         []string
}

// GenerateTypedArgs parses string arguments into a range of types to pass to the ABI call
func GenerateTypedArgs(abi abi.ABI, methodName string, strargs []string, totalBatches, batch, batchSize, index, workerIndex int) ([]interface{}, error) {

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
			if strings.Contains(strval, "HASH:") {
				// special parameter that needs a hash to be calculated based on the index following the prefix
				idxstr := strings.TrimPrefix(strval, "HASH:")
				idx, err := strconv.Atoi(idxstr)
				if err != nil {
					fmt.Printf("Failed to parse index of the HASH: %s\n", strval)
					return nil, fmt.Errorf("Failed to parse index of the HASH: %s. %s", strval, err)
				}
				targetVal := strargs[idx]
				if strings.Contains(targetVal, "RANGE:") {
					resolved, err := resolveRangeArg(targetVal, workerIndex, totalBatches, batchSize, batch, index)
					if err != nil {
						return nil, err
					}
					targetVal = resolved
				}

				h := sha256.New()
				h.Write([]byte(targetVal))
				hash := fmt.Sprintf("%x", h.Sum(nil))
				typedArgs = append(typedArgs, hash)
			} else {
				typedArgs = append(typedArgs, strval)
			}
			break
		case "int256", "uint256":
			if strings.Contains(strval, "RANGE:") {
				resolved, err := resolveRangeArg(strval, workerIndex, totalBatches, batchSize, batch, index)
				if err != nil {
					return nil, err
				}
				strval = resolved
			}
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
func CompileContract(solidityFile, evmVersion, contractName, method string, args []string) (*CompiledSolidity, error) {
	var c CompiledSolidity

	solcVer, err := compiler.SolidityVersion("solc")
	if err != nil {
		return nil, fmt.Errorf("Failed to find solidity version: %s", err)
	}
	solcArgs := []string{
		"--combined-json", "bin,bin-runtime,srcmap,srcmap-runtime,abi,userdoc,devdoc,metadata",
		"--optimize",
		"--evm-version", evmVersion,
		"--allow-paths", ".",
		solidityFile,
	}
	solOptionsString := strings.Join(append([]string{solcVer.Path}, solcArgs...), " ")
	log.Debugf("Compiling: %s", solOptionsString)
	cmd := exec.Command(solcVer.Path, solcArgs...)

	// Compile the solidity
	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("Failed to compile [%s]: %s", err, stderr.String())
	}

	compiled, err := compiler.ParseCombinedJSON(stdout.Bytes(), "", solcVer.Version, solcVer.Version, solOptionsString)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse solc output: %s", err)
	}

	// Check we only have one conract and grab the code/info
	var contract *compiler.Contract
	contractNames := reflect.ValueOf(compiled).MapKeys()
	if contractName != "" {
		if _, ok := compiled[contractName]; !ok {
			return nil, fmt.Errorf("Contract %s not found in Solidity file: %s", contractName, contractNames)
		}
		contract = compiled[contractName]
	} else if len(contractNames) != 1 {
		return nil, fmt.Errorf("More than one contract in Solidity file, please set one to call: %s", contractNames)
	} else {
		contractName = contractNames[0].String()
		contract = compiled[contractName]
	}
	c.ContractInfo = contract.Info
	c.Compiled = contract.Code
	c.method = method
	c.args = args

	// Pack the arguments for calling the contract
	abiJSON, err := json.Marshal(contract.Info.AbiDefinition)
	if err != nil {
		return nil, fmt.Errorf("Serializing ABI: %s", err)
	}
	abi, err := abi.JSON(bytes.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("Parsing ABI: %s", err)
	}

	c.abi = abi

	return &c, nil
}

func (c *CompiledSolidity) PackCall(totalBatches, batch, batchSize, index, workerIndex int) ([]byte, error) {
	typedArgs, err := GenerateTypedArgs(c.abi, c.method, c.args, totalBatches, batch, batchSize, index, workerIndex)
	if err != nil {
		return nil, err
	}
	packedCall, err := c.abi.Pack(c.method, typedArgs...)
	if err != nil {
		return nil, fmt.Errorf("Packing arguments %s for call %s: %s", c.args, c.method, err)
	}
	return packedCall, nil
}

func resolveRangeArg(strval string, workerIndex, totalBatches, batchSize, batch, index int) (string, error) {
	start := strings.TrimPrefix(strval, "RANGE:")
	rand.Seed(time.Now().UnixNano())
	min, err := strconv.Atoi(start)
	if err != nil {
		fmt.Printf("Failed to parse start of the RANGE: %s\n", strval)
		return "", fmt.Errorf("Failed to parse start of the RANGE: %s. %s", strval, err)
	}
	strval = strconv.Itoa(min + workerIndex*totalBatches*batchSize + batch*batchSize + index)
	fmt.Printf("Calculated RANGE index for worker %d patch %d index %d: %s\n", workerIndex, batch, index, strval)
	return strval, nil
}
