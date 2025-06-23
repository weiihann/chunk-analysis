package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/weiihann/chunk-analysis/internal"
	"github.com/weiihann/chunk-analysis/internal/logger"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the chunk analysis",
	Long:  `Run the chunk analysis`,
	Run:   executeRun,
}

func executeRun(cmd *cobra.Command, args []string) {
	log := logger.GetLogger("run")

	config, err := internal.LoadConfig("./configs")
	if err != nil {
		log.Error("Configuration validation failed", "error", err)
		os.Exit(1)
	}
	log.Info("Configuration loaded", "config", config.String())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())

	var clients []*internal.RpcClient
	for _, rpcURL := range config.RPCURLs {
		rpcClient, err := internal.NewRpcClient(rpcURL, ctx)
		if err != nil {
			log.Error("Failed to create RPC client", "error", err)
			os.Exit(1)
		}
		defer rpcClient.Close()
		clients = append(clients, rpcClient)
	}

	for _, client := range clients {
		res, err := client.Code("0x6d3481c2bf5d9427226406f47b5fe6f6751437b0")
		if err != nil {
			log.Error("Failed to trace block", "error", err)
			os.Exit(1)
		}
		fmt.Println(res)
	}

	// <-sigChan
	log.Info("Received shutdown signal, stopping all services...")
	cancel()
}
