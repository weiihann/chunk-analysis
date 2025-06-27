package internal

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/weiihann/chunk-analysis/internal/logger"
)

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      bool
}

type RpcClient struct {
	ctx         context.Context
	client      *rpc.Client
	retryConfig RetryConfig
	log         *slog.Logger
}

func NewRpcClient(url string, ctx context.Context, config *Config) (*RpcClient, error) {
	client, err := rpc.DialContext(ctx, url)
	if err != nil {
		return nil, err
	}

	retryConfig := RetryConfig{
		MaxAttempts: config.RetryMaxAttempts,
		BaseDelay:   time.Duration(config.RetryBaseDelay) * time.Millisecond,
		MaxDelay:    time.Duration(config.RetryMaxDelay) * time.Millisecond,
		Jitter:      config.RetryJitter,
	}

	return &RpcClient{
		ctx:         ctx,
		client:      client,
		retryConfig: retryConfig,
		log:         logger.GetLogger("rpcclient"),
	}, nil
}

// TraceConfig represents the configuration for debug_traceBlockByNumber
type TraceConfig struct {
	DisableMemory  bool `json:"disableMemory"`
	DisableStorage bool `json:"disableStorage"`
}

// TransactionTrace represents the raw JSON structure from debug_traceBlockByNumber
type TransactionTrace struct {
	TxHash string      `json:"txHash"`
	Result InnerResult `json:"result"`
}

// InnerResult represents the raw result structure
type InnerResult struct {
	Steps  []TraceStep `json:"structLogs"`
	Failed bool        `json:"failed"`
	// Gas         uint64      `json:"gas"`
	// ReturnValue string      `json:"returnValue"`
}

type TraceStep struct {
	PC    uint64   `json:"pc"`    // Program counter
	Op    string   `json:"op"`    // Opcode name
	Depth int      `json:"depth"` // Call stack depth
	Stack []string `json:"stack"` // Stack
	// Gas     uint64 `json:"gas"`     // Remaining gas
	// GasCost uint64 `json:"gasCost"` // Gas cost for this operation
}

func (c *RpcClient) TraceBlockByNumber(blockNum uint64) ([]TransactionTrace, error) {
	bnHex := hexutil.EncodeUint64(blockNum)

	var result []TransactionTrace
	err := c.withRetry(func() error {
		return c.client.CallContext(c.ctx, &result, "debug_traceBlockByNumber", bnHex, TraceConfig{
			DisableMemory:  true,
			DisableStorage: true,
		})
	}, fmt.Sprintf("TraceBlockByNumber(%d)", blockNum))
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Only get the to address, which is the contract address to be analyzed
type TxByHash struct {
	To string `json:"to"`
}

func (c *RpcClient) TransactionByHash(hash string) (TxByHash, error) {
	var result TxByHash
	err := c.withRetry(func() error {
		return c.client.CallContext(c.ctx, &result, "eth_getTransactionByHash", hash)
	}, fmt.Sprintf("TransactionByHash(%s)", hash))
	if err != nil {
		return TxByHash{}, err
	}

	return result, nil
}

func (c *RpcClient) Code(address common.Address, blockNum uint64) (string, error) {
	var result string
	err := c.withRetry(func() error {
		return c.client.CallContext(c.ctx, &result, "eth_getCode", address, hexutil.EncodeUint64(blockNum))
	}, fmt.Sprintf("Code(%s, %d)", address.Hex(), blockNum))
	if err != nil {
		return "", err
	}

	return result, nil
}

func (c *RpcClient) Close() {
	c.client.Close()
}

// withRetry executes the given function with exponential backoff and jitter
func (c *RpcClient) withRetry(fn func() error, operation string) error {
	var lastErr error

	for attempt := 1; attempt <= c.retryConfig.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			if attempt > 1 {
				c.log.Info("RPC call succeeded after retry",
					"operation", operation,
					"attempt", attempt,
				)
			}
			return nil
		}

		lastErr = err

		if attempt == c.retryConfig.MaxAttempts {
			c.log.Error("RPC call failed after all retries",
				"operation", operation,
				"attempts", attempt,
				"error", err,
			)
			break
		}

		delay := c.calculateDelay(attempt)
		c.log.Warn("RPC call failed, retrying",
			"operation", operation,
			"attempt", attempt,
			"maxAttempts", c.retryConfig.MaxAttempts,
			"delay", delay,
			"error", err,
		)

		select {
		case <-c.ctx.Done():
			return fmt.Errorf("context cancelled during retry: %w", c.ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("RPC call failed after %d attempts: %w", c.retryConfig.MaxAttempts, lastErr)
}

// calculateDelay calculates the delay for the given attempt with exponential backoff and optional jitter
func (c *RpcClient) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^(attempt-1)
	delay := float64(c.retryConfig.BaseDelay) * math.Pow(2, float64(attempt-1))

	// Apply max delay cap
	if delay > float64(c.retryConfig.MaxDelay) {
		delay = float64(c.retryConfig.MaxDelay)
	}

	// Apply jitter if enabled
	if c.retryConfig.Jitter {
		// Add random jitter between 0% and 50% of the delay
		jitter := rand.Float64() * 0.5 * delay
		delay += jitter
	}

	return time.Duration(delay)
}
