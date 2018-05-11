package cmd

import (
	"fmt"
	"os"

	"github.com/kaleido-io/kaleido-go/pkg/kldexerciser"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func initLogging(debugLevel int) {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.Info("Debug sets ", debugLevel)
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
}

var exerciserConfig kldexerciser.KldExerciserConfig

func init() {
	cmd.Flags().StringVarP(&exerciserConfig.URL, "url", "u", "", "JSON/RPC URL for the Ethereum node: https://user:pass@xyz-rpc.kaleido.io")
	cmd.Flags().StringVarP(&exerciserConfig.Contract, "contract", "c", "", "Contract address (with or without 0x prefix)")
	cmd.Flags().IntVarP(&exerciserConfig.Txns, "transactions", "t", 1, "Count of transactions to run per worker (0=infinite)")
	cmd.Flags().IntVarP(&exerciserConfig.Workers, "workers", "w", 1, "Number of workers to run")
	cmd.Flags().IntVarP(&exerciserConfig.DebugLevel, "debug", "d", 1, "0=error, 1=info, 2=debug")
	cmd.MarkFlagRequired("url")
	cmd.MarkFlagRequired("contract")
	viper.SetDefault("debug", 1)
	viper.SetDefault("count", 1)
	viper.SetDefault("workers", 1)
}

var cmd = &cobra.Command{
	Use:   "kaleido-go",
	Short: "Sample exerciser for Ethereum permissioned chains - from Kaleido",
	Run: func(cmd *cobra.Command, args []string) {
		initLogging(viper.GetInt("debug"))
	},
}

// Execute is called by the main method of the package
func Execute() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
