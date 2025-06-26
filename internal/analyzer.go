package internal

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/weiihann/chunk-analysis/internal/logger"
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

	results chan<- *TraceResult
}

type TraceResult struct {
	Addr string
	Bits *BitSet

	// These opcodes accesses the entire contract code, keep them separate so we can distinguish between
	// actual code access from the other opcodes versus just these ones.
	// 0 means no call to this opcode was made.
	CodeSizeCount int
	CodeCopyCount int
	CodeHashCount int
}

type Code struct {
	addr string
	code []byte
}

func newTraceResult(code *Code) *TraceResult {
	return &TraceResult{
		Addr: code.addr,
		Bits: NewBitSet(uint32(len(code.code))),
	}
}

func NewAnalyzer(client *RpcClient, retriever *TraceRetriever, results chan<- *TraceResult) *Analyzer {
	return &Analyzer{
		client:    client,
		retriever: retriever,
		log:       logger.GetLogger("analyzer"),
		results:   results,
	}
}

func (a *Analyzer) Analyze(blockNum uint64) error {
	trace, err := a.retriever.GetTrace(blockNum)
	if err != nil {
		return err
	}

	for _, tx := range trace {
		if err := a.analyze(&tx, blockNum); err != nil {
			return err
		}
	}

	return nil
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
	code, err := a.client.Code(common.HexToAddress(addr), blockNum)
	if err != nil {
		return nil, err
	}
	codeBytes, err := hexutil.Decode(code)
	if err != nil {
		return nil, err
	}

	return &Code{
		addr: addr,
		code: codeBytes,
	}, nil
}

func (a *Analyzer) analyzeSteps(blockNum uint64, code *Code, trace *InnerResult, depth int, counter *int) error {
	steps := trace.Steps
	if len(steps) == 0 {
		return nil
	}

	// Sanity check on the depth
	if depth != steps[*counter].Depth {
		return fmt.Errorf("(%d) depth mismatch: expected %d, got %d", *counter, depth, steps[*counter].Depth)
	}

	result := newTraceResult(code)
	for *counter < len(steps) {
		step := steps[*counter]

		// We detect that we went back to the previous depth, so this is the end of the current depth
		if step.Depth == depth-1 {
			break
		}

		result.Bits.Set(uint32(step.PC))

		switch op := step.Op; {
		case op == OpPush0:
			// Do nothing
		case strings.HasPrefix(op, "PUSH"):
			if err := a.handlePush(result.Bits, &step); err != nil {
				return err
			}
		case op == OpCodeSize:
			result.CodeSizeCount++
		case op == OpCodeCopy:
			result.CodeCopyCount++
		case op == OpExtCodeSize:
			code, err := a.getCode(step.Stack[len(step.Stack)-1], blockNum)
			if err != nil && !trace.Failed {
				return err
			}
			if len(code.code) != 0 {
				extRes := newTraceResult(code)
				extRes.CodeSizeCount++
				a.results <- extRes
			}
		case op == OpExtCodeHash:
			code, err := a.getCode(step.Stack[len(step.Stack)-1], blockNum)
			if err != nil && !trace.Failed {
				return err
			}
			if len(code.code) != 0 {
				extRes := newTraceResult(code)
				extRes.CodeHashCount++
				a.results <- extRes
			}
		case op == OpExtCodeCopy:
			code, err := a.getCode(step.Stack[len(step.Stack)-1], blockNum)
			if err != nil && !trace.Failed {
				return err
			}
			if len(code.code) != 0 {
				extRes := newTraceResult(code)
				extRes.CodeCopyCount++
				a.results <- extRes
			}
		case op == OpDelegateCall || op == OpCall || op == OpCallCode || op == OpStaticCall:
			code, err := a.getCode(step.Stack[len(step.Stack)-2], blockNum)
			if err != nil && !trace.Failed {
				return err
			}
			if len(code.code) != 0 {
				*counter++
				if err := a.analyzeSteps(blockNum, code, trace, depth+1, counter); err != nil {
					return err
				}
			}
		}

		*counter++
	}

	a.results <- result
	return nil
}

// PUSHX opcodes also access the bytecode, add it to the result accordingly
func (a *Analyzer) handlePush(bits *BitSet, step *TraceStep) error {
	pushNum := step.Op[len("PUSH"):] // Extract the PUSHN number
	pushNumInt, err := strconv.Atoi(pushNum)
	if err != nil {
		return err
	}
	for i := range pushNumInt {
		res, err := bits.SetWithCheck(uint32(step.PC) + 1 + uint32(i))
		if err != nil {
			return err
		}
		bits = res
	}
	return nil
}
