package cmd

import (
	"context"
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
	defer cancel()

	engine := internal.NewEngine(&config)

	// Run engine in a goroutine so we can handle signals during execution
	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.Run(ctx)
		log.Info("Engine finished processing all blocks")
	}()

	// Wait for either completion or signal
	select {
	case <-done:
		log.Info("Analysis completed successfully, shutting down...")
	case <-sigChan:
		log.Info("Received shutdown signal, stopping all services...")
		cancel()
		<-done // Wait for engine to finish cleanup
	}
}
