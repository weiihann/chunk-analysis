package internal

import (
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"sync"

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
	codeCache *lru.Cache // This should be shared, or just put into the rpc client
}

type TraceResult struct {
	Addr common.Address
	Bits *BitSet
	Skip bool // Skip this result if it's either a create or self destruct

	// These opcodes access the entire contract code, keep them separate so we can distinguish between
	// actual code access from the other opcodes versus just these ones.
	// 0 means no call to this opcode was made.
	CodeSizeCount int // CODESIZE, EXTCODESIZE
	CodeCopyCount int // CODECOPY, EXTCODECOPY
}

func (t *TraceResult) String() string {
	if t.Skip {
		return fmt.Sprintf("Addr: %s, Skip: true", t.Addr.Hex())
	}
	return fmt.Sprintf("Addr: %s, Bits: %d, Chunks: %d, CodeSizeCount: %d, CodeCopyCount: %d",
		t.Addr.Hex(),
		t.Bits.Count(),
		t.Bits.ChunkCount(),
		t.CodeSizeCount,
		t.CodeCopyCount,
	)
}

type Code struct {
	addr common.Address
	code []byte
}

func newTraceResult(code *Code) *TraceResult {
	return &TraceResult{
		Addr: code.addr,
		Bits: NewBitSet(uint32(len(code.code))),
	}
}

func newTraceResultSkip() *TraceResult {
	return &TraceResult{
		Skip: true,
	}
}

func NewAnalyzer(id int, client *RpcClient, retriever *TraceRetriever, codeCache *lru.Cache) *Analyzer {
	return &Analyzer{
		client:    client,
		retriever: retriever,
		log:       logger.GetLogger(fmt.Sprintf("analyzer-%d", id)),
		codeCache: codeCache,
	}
}

type BlockResult struct {
	BlockNum uint64
	Results  map[common.Address]*MergedTraceResult
}

type MergedTraceResult struct {
	Bits          *BitSet
	CodeSizeCount int
	CodeCopyCount int
}

func (a *Analyzer) Analyze(blockNum uint64, trace []TransactionTrace) (BlockResult, error) {
	// Aggregate the results and send it back
	// Merge all results per contract
	aggregated := make(map[common.Address]*MergedTraceResult)
	var mu sync.Mutex
	merge := func(result map[common.Address]*TraceResult) {
		mu.Lock()
		defer mu.Unlock()
		for addr, res := range result {
			if existing, exists := aggregated[addr]; exists {
				existing.Bits.Merge(res.Bits)
				existing.CodeSizeCount += res.CodeSizeCount
				existing.CodeCopyCount += res.CodeCopyCount
			} else {
				aggregated[addr] = &MergedTraceResult{
					Bits:          res.Bits,
					CodeSizeCount: res.CodeSizeCount,
					CodeCopyCount: res.CodeCopyCount,
				}
			}
		}
	}

	// ---- Uncomment below to debug
	var workers errgroup.Group
	workers.SetLimit(runtime.NumCPU())
	for _, tx := range trace {
		workers.Go(func() error {
			// fmt.Printf("analyzing tx %d\n", i)
			res, err := a.analyze(&tx, blockNum)
			if err != nil {
				return err
			}
			merge(res)
			return nil
		})
	}

	if err := workers.Wait(); err != nil {
		return BlockResult{}, err
	}

	// ---- Uncomment below to debug
	// for i, tx := range trace {
	// 	fmt.Printf("analyzing tx %d\n", i)
	// 	res, err := a.analyze(&tx, blockNum)
	// 	if err != nil {
	// 		return BlockResult{}, err
	// 	}
	// 	merge(res)
	// }

	// ---- Uncomment below to debug
	// targetTrace := trace[141]
	// fmt.Println(targetTrace.TxHash)
	// res, err := a.analyze(&targetTrace, blockNum)
	// if err != nil {
	// 	return BlockResult{}, err
	// }
	// merge(res)

	return BlockResult{
		BlockNum: blockNum,
		Results:  aggregated,
	}, nil
}

func (a *Analyzer) analyze(tr *TransactionTrace, blockNum uint64) (map[common.Address]*TraceResult, error) {
	code, err := a.getCodeFromTx(tr.TxHash, blockNum)
	if err != nil {
		return nil, err
	}

	if len(code.code) == 0 {
		return nil, nil
	}

	codes := make(map[int][]*TraceResult)
	codes[1] = []*TraceResult{newTraceResult(code)}

	res, err := a.analyzeSteps(blockNum, &tr.Result, codes)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (a *Analyzer) getCodeFromTx(txHash string, blockNum uint64) (*Code, error) {
	tx, err := a.client.TransactionByHash(txHash)
	if err != nil {
		return nil, err
	}

	return a.getCode(tx.To, blockNum)
}

func (a *Analyzer) getCode(addr string, blockNum uint64) (*Code, error) {
	addrHex := common.HexToAddress(addr)
	cacheKey := codeCacheKey(addrHex, blockNum)
	if cached, ok := a.codeCache.Get(cacheKey); ok {
		return cached.(*Code), nil
	}

	code, err := a.client.Code(addrHex, blockNum)
	if err != nil {
		return nil, err
	}
	codeBytes, err := hexutil.Decode(code)
	if err != nil {
		return nil, err
	}

	result := &Code{
		addr: addrHex,
		code: codeBytes,
	}
	a.codeCache.Add(cacheKey, result)
	return result, nil
}

func (a *Analyzer) analyzeSteps(blockNum uint64, trace *InnerResult, codes map[int][]*TraceResult) (map[common.Address]*TraceResult, error) {
	results := make(map[common.Address]*TraceResult)
	results[codes[1][0].Addr] = codes[1][0]

	for i, step := range trace.Steps {
		// if i == 2954 { // TODO: remove
		// 	a.log.Info("step 2954")
		// }
		// fmt.Printf("step %d: pc %d, op %s depth %d\n", i, step.PC, step.Op, step.Depth) // TODO: remove
		op := step.Op
		opLen := len(op)
		stack := step.Stack
		switch {
		// EXTCODESIZE, EXTCODEHASH, EXTCODECOPY
		case opLen == 11 && op[0] == 'E':
			stackTop := step.Stack[len(step.Stack)-1]
			code, err := a.getCode(stackTop, blockNum)
			if err != nil && !trace.Failed {
				return nil, err
			}
			if len(code.code) != 0 {
				if _, ok := results[code.addr]; !ok {
					results[code.addr] = newTraceResult(code)
				}
				switch op[len(op)-1] {
				case 'Y':
					results[code.addr].CodeCopyCount++
				case 'E':
					results[code.addr].CodeSizeCount++
				}
			}
		// CALL, STATICCALL, DELEGATECALL, CALLCODE
		case (opLen == 4 && op[3] == 'L') || (opLen == 10 && op[9] == 'L') || (opLen == 12 && op[0] == 'D') || (opLen == 8 && op[2] == 'L'):
			if i+1 < len(trace.Steps) && trace.Steps[i+1].Depth == step.Depth+1 {
				nextStep := trace.Steps[i+1]
				code, err := a.getCode(stack[len(stack)-2], blockNum)
				if err != nil && !trace.Failed {
					return nil, err
				}
				if len(code.code) != 0 {
					res, ok := results[code.addr]
					if !ok {
						res = newTraceResult(code)
						results[code.addr] = res
					}
					codes[nextStep.Depth] = append(codes[nextStep.Depth], res)
				} else { // SELFDESTRUCT
					nextStep := trace.Steps[i+1]
					codes[nextStep.Depth] = append(codes[nextStep.Depth], newTraceResultSkip())
				}
			}
		case opLen >= 6 && op[:2] == "CR": // CREATE, CREATE2
			if i+1 < len(trace.Steps) && trace.Steps[i+1].Depth == step.Depth+1 {
				nextStep := trace.Steps[i+1]
				codes[nextStep.Depth] = append(codes[nextStep.Depth], newTraceResultSkip())
			}
		}
	}

	// TODO: remove
	// for depth, res := range codes {
	// 	fmt.Printf("depth %d: %v\n", depth, res)
	// }
	// for addr, res := range results {
	// 	fmt.Printf("addr %s: %v\n", addr.Hex(), res)
	// }

	// Populate the initial pointers for each depth
	pts := make(map[int]int)
	for depth := range codes {
		pts[depth] = 0
	}

	// Second iteration, populate the results accordingly.
	var prevDepth int
	for _, step := range trace.Steps {
		// fmt.Printf("step %d: pc %d, op %s depth %d stack %v\n", i, step.PC, step.Op, step.Depth, step.Stack) // TODO: remove
		// if i == 2954 {
		// 	a.log.Info("step 3985")
		// }
		op := step.Op
		opLen := len(op)
		depth := step.Depth

		if prevDepth > depth {
			pts[prevDepth]++
		}

		res := codes[depth][pts[depth]]
		if res.Skip {
			prevDepth = depth
			continue
		}

		switch {
		case opLen == 4 && op[:2] == "ST": // STOP
			prevDepth = depth
			if step.PC < uint64(res.Bits.Size()) {
				res.Bits.Set(uint32(step.PC))
			}
			continue
		case op == OpPush0:
			// Do nothing
		case opLen > 4 && op[:2] == "PU": // PUSH opcodes
			if err := a.handlePush(res.Bits, &step); err != nil {
				return nil, err
			}
		case opLen > 4 && op[:3] == "COD": // CODESIZE, CODECOPY
			switch op[len(op)-1] {
			case 'Y':
				res.CodeCopyCount++
			case 'E':
				res.CodeSizeCount++
			}
		}

		prevDepth = depth
		res.Bits.Set(uint32(step.PC))
	}

	return results, nil
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
		bits.Set(pc + 1 + uint32(i))
	}
	return nil
}

func codeCacheKey(addr common.Address, blockNum uint64) string {
	return fmt.Sprintf("%s:%d", addr.Hex(), blockNum)
}
