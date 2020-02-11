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

package cmd

import (
	"fmt"
	"os"

	"github.com/kaleido-io/kaleido-go/pkg/kldexerciser"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func initLogging(debugLevel int) {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	switch debugLevel {
	case 0:
		log.SetLevel(log.ErrorLevel)
	case 1:
		log.SetLevel(log.InfoLevel)
	default:
		debugLevel = 2
		log.SetLevel(log.DebugLevel)
	}
	log.Debug("Debug level ", debugLevel)
}

var exerciser kldexerciser.Exerciser

func init() {
	cmd.Flags().StringArrayVarP(&exerciser.Accounts, "accounts", "a", []string{}, "Account addresses - 1 per worker needed for geth signing")
	cmd.Flags().StringArrayVarP(&exerciser.Args, "args", "x", []string{}, "String arguments to pass to contract method (auto-converted to type)")
	cmd.Flags().Int64VarP(&exerciser.ChainID, "chainid", "i", 0, "Chain ID for EIP155 signing (networkid queried if omitted)")
	cmd.Flags().BoolVarP(&exerciser.Call, "call", "C", false, "Call the contract and return a value, rather than sending a txn")
	cmd.Flags().StringVarP(&exerciser.Contract, "contract", "c", "", "Pre-deployed contract address. Will be deployed if not specified")
	cmd.Flags().StringVarP(&exerciser.ContractName, "contractname", "n", "", "The name of the contract to call, for Solidity files with multiple contracts")
	cmd.Flags().IntVarP(&exerciser.DebugLevel, "debug", "d", 1, "0=error, 1=info, 2=debug")
	cmd.Flags().Int64VarP(&exerciser.Nonce, "nonce", "N", -1, "Nonce (transaction number) for the next transaction")
	cmd.Flags().BoolVarP(&exerciser.ExternalSign, "extsign", "e", false, "Sign externally with generated private keys + accounts")
	cmd.Flags().StringVarP(&exerciser.ExternalSignJSON, "keys", "k", "", "JSON file to create/update with an array of private keys for extsign")
	cmd.Flags().BoolVarP(&exerciser.EstimateGas, "estimategas", "E", false, "Estimate the gas for the contract call, rather than sending a txn")
	cmd.Flags().StringVarP(&exerciser.EVMVersion, "evm-version", "V", "byzantium", "EVM version to compile for (byzantium etc.)")
	cmd.Flags().StringVarP(&exerciser.SolidityFile, "file", "f", "", "Solidity smart contract source. Deployed if --contract not supplied")
	cmd.Flags().Int64VarP(&exerciser.StatsdFlushPeriod, "flush-period", "F", 1000, "Flush period for statsd metrics (ms)")
	cmd.Flags().Int64VarP(&exerciser.Gas, "gas", "g", 1000000, "Gas limit on the transaction")
	cmd.Flags().Int64VarP(&exerciser.GasPrice, "gasprice", "G", 0, "Gas price")
	cmd.Flags().IntVarP(&exerciser.Loops, "loops", "l", 1, "Loops to perform in each worker before exiting (0=infinite)")
	cmd.Flags().StringVarP(&exerciser.Method, "method", "m", "", "Method name in the contract to invoke")
	cmd.Flags().StringArrayVarP(&exerciser.PrivateFor, "privateFor", "P", []string{}, "Private for (see EEA Client Spec V1)")
	cmd.Flags().StringVarP(&exerciser.PrivateFrom, "privateFrom", "p", "", "Private from (see EEA Client Spec V1)")
	cmd.Flags().IntVarP(&exerciser.RPCTimeout, "rpc-timeout", "R", 30, "Timeout in seconds for an individual RCP call")
	cmd.Flags().IntVarP(&exerciser.ReceiptWaitMin, "seconds-min", "s", 11, "Time in seconds to wait before checking for a txn receipt/before making subsequent contract call")
	cmd.Flags().IntVarP(&exerciser.ReceiptWaitMax, "seconds-max", "S", 20, "Time in seconds before timing out waiting for a txn receipt")
	cmd.Flags().StringVarP(&exerciser.StatsdServer, "metrics", "M", "", "statsd server to submit metrics to")
	cmd.Flags().IntVarP(&exerciser.TxnsPerLoop, "transactions", "t", 1, "Count of transactions submit on each worker loop")
	cmd.Flags().BoolVarP(&exerciser.StatsdTelegraf, "telegraf", "T", false, "Telegraf/InfluxDB stats naming (default is Graphite)")
	cmd.Flags().StringVarP(&exerciser.StatsdQualifier, "metrics-qualifier", "q", "", "Additional metrics qualifier")
	cmd.Flags().StringVarP(&exerciser.URL, "url", "u", "", "JSON/RPC URL for Ethereum node: https://user:pass@xyz-rpc.kaleido.io")
	cmd.Flags().IntVarP(&exerciser.Workers, "workers", "w", 1, "Number of workers to run")
	cmd.MarkFlagRequired("url")
	cmd.MarkFlagRequired("file")
	cmd.MarkFlagRequired("method")
}

var cmd = &cobra.Command{
	Use:   "kaleido-go",
	Short: "Sample exerciser for Ethereum permissioned chains - from Kaleido",
	Run: func(cmd *cobra.Command, args []string) {
		initLogging(exerciser.DebugLevel)
		if err := exerciser.Start(); err != nil {
			log.Error("Exerciser Start: ", err)
			os.Exit(1)
		}
	},
}

// Execute is called by the main method of the package
func Execute() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
