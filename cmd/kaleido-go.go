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
		break
	case 1:
		log.SetLevel(log.InfoLevel)
		break
	default:
		log.SetLevel(log.DebugLevel)
		break
	}
	log.Debug("Debug level ", debugLevel)
}

var exerciser kldexerciser.Exerciser

func init() {
	cmd.Flags().IntVarP(&exerciser.DebugLevel, "debug", "d", 1, "0=error, 1=info, 2=debug")
	cmd.Flags().StringVarP(&exerciser.URL, "url", "u", "", "JSON/RPC URL for the Ethereum node: https://user:pass@xyz-rpc.kaleido.io")
	cmd.Flags().StringVarP(&exerciser.SolidityFile, "file", "f", "", "Solidity smart contract to compile (and deploy if no contract address supplied)")
	cmd.Flags().StringVarP(&exerciser.Contract, "contract", "c", "", "Pre-deployed contract address. Will be deployed if not specified")
	cmd.Flags().StringVarP(&exerciser.Method, "method", "m", "", "Method in the contract to invoke")
	cmd.Flags().StringArrayVarP(&exerciser.Args, "args", "x", []string{}, "String arguments to pass to contract method")
	cmd.Flags().IntVarP(&exerciser.TxnsPerLoop, "transactions", "t", 1, "Count of transactions submit on each worker loop")
	cmd.Flags().Int64VarP(&exerciser.Gas, "gas", "g", 1000000, "Gas limit on the transaction")
	cmd.Flags().Int64VarP(&exerciser.GasPrice, "gasprice", "G", 0, "Gas price")
	cmd.Flags().IntVarP(&exerciser.Loops, "loops", "l", 1, "How many times each looper should loop till exiting (0=infinite)")
	cmd.Flags().IntVarP(&exerciser.ReceiptWaitMin, "seconds-min", "s", 11, "Time in seconds to wait before checking for a txn receipt")
	cmd.Flags().IntVarP(&exerciser.ReceiptWaitMax, "seconds-max", "S", 20, "Time in seconds before timing out waiting for a txn receipt")
	cmd.Flags().IntVarP(&exerciser.Workers, "workers", "w", 1, "Number of workers to run")
	cmd.Flags().BoolVarP(&exerciser.ExternalSign, "extsign", "e", false, "Sign externally with generated accounts")
	cmd.Flags().Int64VarP(&exerciser.ChainID, "chainid", "i", 0, "Chain ID - required for external signing")
	cmd.Flags().StringArrayVarP(&exerciser.Accounts, "accounts", "a", []string{}, "Account addresses - 1 per worker needed for internal geth client signing")
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
