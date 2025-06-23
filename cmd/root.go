package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "chunk-analysis",
	Short: "A CLI to analyze bytecode chunks on Ethereum mainnet",
	Long:  `Chunk-analysis is a CLI tool that allows you to analyze bytecode chunks on Ethereum mainnet.`,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
