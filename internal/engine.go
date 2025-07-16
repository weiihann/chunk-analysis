package internal

import (
	"context"
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
	// Set chunk size (definitely not a good practice)
	chunkSize = e.config.ChunkSize
	e.log.Info("chunk size", "chunk_size", chunkSize)

	analyzers := e.prepare(ctx)

	// Split the analyzers into different chunks
	// Calculate total blocks and distribute evenly among workers
	var workers errgroup.Group

	startBlocks := e.config.StartBlocks
	endBlocks := e.config.EndBlocks

	if len(startBlocks) != len(endBlocks) && len(startBlocks) != len(analyzers) {
		panic("startBlocks and endBlocks must have the same length as analyzers")
	}

	blockInc := (e.config.GlobalEndBlock - e.config.GlobalStartBlock + 1) / e.config.SampleSize

	for i := 0; i < len(analyzers); i++ {
		workerIdx := i
		worker := analyzers[i]

		start := startBlocks[workerIdx]
		end := endBlocks[workerIdx]

		workers.Go(func() error {
			e.log.Info("starting worker", "worker_idx", workerIdx, "start", start, "end", end)
			writer := NewResultWriter(e.config.ResultDir, workerIdx)

			var retrievers errgroup.Group
			traces := make(chan traceResult, 1) // buffered to avoid deadlocks
			retrievers.Go(func() error {
				defer close(traces)
				for block := start; block <= end; block += blockInc {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
						trace, err := worker.retriever.GetTrace(block)
						if err != nil {
							return err
						}
						select {
						case traces <- traceResult{
							blockNum: block,
							trace:    trace,
						}:
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				}
				return nil
			})

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case traceResult, ok := <-traces:
					if !ok {
						// If we broke out of the for loop because channel closed, wait for retrievers
						if err := retrievers.Wait(); err != nil {
							return err
						}
						return nil
					}
					result, err := worker.Analyze(traceResult.blockNum, traceResult.trace)
					if err != nil {
						return err
					}

					if err := writer.Write(traceResult.blockNum, result.Results); err != nil {
						return err
					}

					e.log.Info("worker finished", "idx", workerIdx, "block", traceResult.blockNum)
				}
			}
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

type traceResult struct {
	blockNum uint64
	trace    []TransactionTrace
}
