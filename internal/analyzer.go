package internal

import (
	"fmt"
	"log/slog"
	"runtime"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/hashicorp/golang-lru"
	"github.com/weiihann/chunk-analysis/internal/logger"
	"golang.org/x/sync/errgroup"
)

// Opcode constants for better maintainability
const (
	OpPush0        = "PUSH0"
	OpCodeSize     = "CODESIZE"
	OpCodeCopy     = "CODECOPY"
	OpExtCodeSize  = "EXTCODESIZE"
	OpExtCodeCopy  = "EXTCODECOPY"
	OpExtCodeHash  = "EXTCODEHASH"
	OpDelegateCall = "DELEGATECALL" // address at stack[top-1]
	OpCall         = "CALL"         // address at stack[top-1]
	OpCallCode     = "CALLCODE"     // address at stack[top-1]
	OpStaticCall   = "STATICCALL"   // address at stack[top-1]
)

type Analyzer struct {
	client    *RpcClient
	retriever *TraceRetriever
	log       *slog.Logger
	codeCache *lru.Cache

	results chan<- *TraceResult
}

type TraceResult struct {
	BlockNum uint64
	Addr     string
	Bits     *BitSet

	// These opcodes access the entire contract code, keep them separate so we can distinguish between
	// actual code access from the other opcodes versus just these ones.
	// 0 means no call to this opcode was made.
	CodeOpsCount int // CODESIZE, CODECOPY, EXTCODESIZE, EXTCODEHASH, EXTCODECOPY
}

type Code struct {
	addr string
	code []byte
}

func newTraceResult(code *Code, blockNum uint64) *TraceResult {
	return &TraceResult{
		BlockNum: blockNum,
		Addr:     code.addr,
		Bits:     NewBitSet(uint32(len(code.code))),
	}
}

func NewAnalyzer(id int, client *RpcClient, retriever *TraceRetriever, results chan<- *TraceResult) *Analyzer {
	codeCache, err := lru.New(100000)
	if err != nil {
		panic(err)
	}

	return &Analyzer{
		client:    client,
		retriever: retriever,
		log:       logger.GetLogger(fmt.Sprintf("analyzer-%d", id)),
		codeCache: codeCache,
		results:   results,
	}
}

func (a *Analyzer) Analyze(blockNum uint64) error {
	trace, err := a.retriever.GetTrace(blockNum)
	if err != nil {
		return err
	}

	var workers errgroup.Group
	workers.SetLimit(runtime.NumCPU())
	for _, tx := range trace {
		workers.Go(func() error {
			return a.analyze(&tx, blockNum)
		})
	}

	return workers.Wait()
}

func (a *Analyzer) analyze(tr *TransactionTrace, blockNum uint64) error {
	code, err := a.getCodeFromTx(tr.TxHash, blockNum)
	if err != nil {
		return err
	}

	counter := 0
	if err := a.analyzeSteps(blockNum, code, &tr.Result, 1, &counter); err != nil {
		return err
	}

	return nil
}

func (a *Analyzer) getCodeFromTx(txHash string, blockNum uint64) (*Code, error) {
	tx, err := a.client.TransactionByHash(txHash)
	if err != nil {
		return nil, err
	}

	return a.getCode(tx.To, blockNum)
}

func (a *Analyzer) getCode(addr string, blockNum uint64) (*Code, error) {
	cacheKey := codeCacheKey(addr, blockNum)
	if cached, ok := a.codeCache.Get(cacheKey); ok {
		return cached.(*Code), nil
	}

	code, err := a.client.Code(common.HexToAddress(addr), blockNum)
	if err != nil {
		return nil, err
	}
	codeBytes, err := hexutil.Decode(code)
	if err != nil {
		return nil, err
	}

	result := &Code{
		addr: addr,
		code: codeBytes,
	}
	a.codeCache.Add(cacheKey, result)
	return result, nil
}

func (a *Analyzer) analyzeSteps(blockNum uint64, code *Code, trace *InnerResult, depth int, counter *int) error {
	steps := trace.Steps
	stepsLen := len(steps)
	if stepsLen == 0 {
		return nil
	}

	// Sanity check on the depth
	if depth != steps[*counter].Depth {
		return fmt.Errorf("(%d) depth mismatch: expected %d, got %d", *counter, depth, steps[*counter].Depth)
	}

	result := newTraceResult(code, blockNum)
	for *counter < stepsLen {
		step := steps[*counter]

		// We detect that we went back to the previous depth, so this is the end of the current depth
		if step.Depth == depth-1 {
			break
		}

		result.Bits.Set(uint32(step.PC))

		op := step.Op
		opLen := len(op)

		switch {
		case op == OpPush0:
			// Do nothing
		case opLen > 4 && op[:2] == "PU": // PUSH opcodes
			if err := a.handlePush(result.Bits, &step); err != nil {
				return err
			}
		case opLen > 4 && op[:3] == "COD": // CODEHASH, CODESIZE
			result.CodeOpsCount++
		case opLen == 11 && op[0] == 'E': // EXTCODESIZE, EXTCODEHASH, EXTCODECOPY
			stackTop := step.Stack[len(step.Stack)-1]
			code, err := a.getCode(stackTop, blockNum)
			if err != nil && !trace.Failed {
				return err
			}
			if len(code.code) != 0 {
				extRes := newTraceResult(code, blockNum)
				extRes.CodeOpsCount++
				a.results <- extRes
			}
		case opLen == 4 && op[3] == 'L': // CALL
			if err := a.handleCallOps(step.Stack, depth, counter, blockNum, trace); err != nil {
				return err
			}
		case opLen == 10 && op[9] == 'L': // STATICCALL
			if err := a.handleCallOps(step.Stack, depth, counter, blockNum, trace); err != nil {
				return err
			}
		case opLen == 12 && op[0] == 'D': // DELEGATECALL
			if err := a.handleCallOps(step.Stack, depth, counter, blockNum, trace); err != nil {
				return err
			}
		case opLen == 8 && op[2] == 'L': // CALLCODE
			if err := a.handleCallOps(step.Stack, depth, counter, blockNum, trace); err != nil {
				return err
			}
		}

		*counter++
	}

	a.results <- result
	return nil
}

func (a *Analyzer) handleCallOps(stack []string, depth int, counter *int, blockNum uint64, trace *InnerResult) error {
	stackSecond := stack[len(stack)-2]
	code, err := a.getCode(stackSecond, blockNum)
	if err != nil && !trace.Failed {
		return err
	}
	if len(code.code) != 0 {
		*counter++
		if err := a.analyzeSteps(blockNum, code, trace, depth+1, counter); err != nil {
			return err
		}
	}
	return nil
}

// PUSHX opcodes also access the bytecode, add it to the result accordingly
func (a *Analyzer) handlePush(bits *BitSet, step *TraceStep) error {
	pushNum := step.Op[4:] // Extract the PUSHN number (skip "PUSH")
	pushNumInt, err := strconv.Atoi(pushNum)
	if err != nil {
		return err
	}

	pc := uint32(step.PC)
	for i := 0; i < pushNumInt; i++ {
		if _, err := bits.SetWithCheck(pc + 1 + uint32(i)); err != nil {
			return err
		}
	}
	return nil
}

func codeCacheKey(addr string, blockNum uint64) string {
	return fmt.Sprintf("%s:%d", addr, blockNum)
}
