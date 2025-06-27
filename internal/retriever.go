package internal

import (
	"encoding/json"
	"fmt"
	"os"
)

type TraceRetriever struct {
	rpcClient *RpcClient
	TraceDir  string
}

func NewTraceRetriever(rpcClient *RpcClient, TraceDir string) *TraceRetriever {
	return &TraceRetriever{
		rpcClient: rpcClient,
		TraceDir:  TraceDir,
	}
}

func (r *TraceRetriever) GetTrace(blockNumber uint64) ([]TransactionTrace, error) {
	traceFile := fmt.Sprintf("%s/block_%d_trace.json", r.TraceDir, blockNumber)
	if _, err := os.Stat(traceFile); err == nil {
		return r.getTraceFromFile(traceFile)
	}

	trace, err := r.rpcClient.TraceBlockByNumber(blockNumber)
	if err != nil {
		return nil, err
	}

	return trace, nil
}

type JSONTrace struct {
	Result []TransactionTrace `json:"result"`
}

func (r *TraceRetriever) getTraceFromFile(filepath string) ([]TransactionTrace, error) {
	trace, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	var jsonTrace JSONTrace
	err = json.Unmarshal(trace, &jsonTrace)
	if err != nil {
		return nil, err
	}
	return jsonTrace.Result, nil
}
