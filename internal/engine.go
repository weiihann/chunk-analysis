package internal

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hashicorp/golang-lru"
	"github.com/weiihann/chunk-analysis/internal/logger"
	"golang.org/x/sync/errgroup"
)

type Engine struct {
	log    *slog.Logger
	config *Config
}

func NewEngine(config *Config) *Engine {
	log := logger.GetLogger("engine")
	return &Engine{
		log:    log,
		config: config,
	}
}

func (e *Engine) Run(ctx context.Context) {
	analyzers := e.prepare(ctx)

	// Split the analyzers into different chunks
	// Calculate total blocks and distribute evenly among workers
	var workers errgroup.Group
	totalBlocks := e.config.EndBlock - e.config.StartBlock + 1
	baseChunkSize := totalBlocks / uint64(len(analyzers))
	remainder := totalBlocks % uint64(len(analyzers))

	currentStart := e.config.StartBlock
	for i := 0; i < len(analyzers); i++ {
		workerIdx := i
		worker := analyzers[i]

		// Calculate chunk size for this worker (distribute remainder to first workers)
		chunkSize := baseChunkSize
		if uint64(workerIdx) < remainder {
			chunkSize++
		}

		start := currentStart
		end := start + chunkSize - 1
		currentStart = end + 1 // Next worker starts after this one ends

		workers.Go(func() error {
			e.log.Info("starting worker", "worker_idx", workerIdx, "start", start, "end", end)
			fmt.Println("starting worker", workerIdx, start, end)
			writer := NewResultWriter(e.config.ResultDir, workerIdx)

			for block := start; block <= end; block++ {
				result, err := worker.Analyze(block)
				if err != nil {
					return err
				}

				if err := writer.Write(block, result.Results); err != nil {
					return err
				}

				e.log.Info("worker finished", "idx", workerIdx, "block", block)
			}
			return nil
		})
	}

	if err := workers.Wait(); err != nil {
		e.log.Error("failed to analyze", "error", err)
	}
}

func (e *Engine) prepare(ctx context.Context) []*Analyzer {
	var analyzers []*Analyzer

	codeCache, err := lru.New(100000)
	if err != nil {
		panic(err)
	}

	for i := 0; i < len(e.config.RPCURLs); i++ {
		client, err := NewRpcClient(e.config.RPCURLs[i], ctx, e.config)
		if err != nil {
			e.log.Error("failed to create rpc client", "error", err)
			continue
		}

		retriever := NewTraceRetriever(client, e.config.TraceDir)

		analyzer := NewAnalyzer(i, client, retriever, codeCache)
		analyzers = append(analyzers, analyzer)
	}

	return analyzers
}
