package internal

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

type RpcClient struct {
	ctx    context.Context
	client *rpc.Client
}

func NewRpcClient(url string, ctx context.Context) (*RpcClient, error) {
	client, err := rpc.DialContext(ctx, url)
	if err != nil {
		return nil, err
	}

	return &RpcClient{
		ctx:    ctx,
		client: client,
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
	if err := c.client.CallContext(c.ctx, &result, "debug_traceBlockByNumber", bnHex, TraceConfig{
		DisableMemory:  true,
		DisableStorage: true,
	}); err != nil {
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
	if err := c.client.CallContext(c.ctx, &result, "eth_getTransactionByHash", hash); err != nil {
		return TxByHash{}, err
	}

	return result, nil
}

func (c *RpcClient) Code(address common.Address, blockNum uint64) (string, error) {
	var result string
	if err := c.client.CallContext(c.ctx, &result, "eth_getCode", address, hexutil.EncodeUint64(blockNum)); err != nil {
		return "", err
	}

	return result, nil
}

func (c *RpcClient) Close() {
	c.client.Close()
}
