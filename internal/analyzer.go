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
	codeCache *lru.Cache // This should be shared, or just put into the rpc client
}

type TraceResult struct {
	BlockNum uint64
	Addr     common.Address
	Bits     *BitSet

	// These opcodes access the entire contract code, keep them separate so we can distinguish between
	// actual code access from the other opcodes versus just these ones.
	// 0 means no call to this opcode was made.
	CodeOpsCount int // CODESIZE, CODECOPY, EXTCODESIZE, EXTCODEHASH, EXTCODECOPY
}

func (t *TraceResult) String() string {
	return fmt.Sprintf("BlockNum: %d, Addr: %s, Bits: %d, Chunks: %d, CodeOpsCount: %d", t.BlockNum, t.Addr.Hex(), t.Bits.Count(), t.Bits.ChunkCount(), t.CodeOpsCount)
}

type Code struct {
	addr common.Address
	code []byte
}

func newTraceResult(code *Code, blockNum uint64) *TraceResult {
	return &TraceResult{
		BlockNum: blockNum,
		Addr:     code.addr,
		Bits:     NewBitSet(uint32(len(code.code))),
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
	Bits         *BitSet
	CodeOpsCount int
}

func (a *Analyzer) Analyze(blockNum uint64) (BlockResult, error) {
	trace, err := a.retriever.GetTrace(blockNum)
	if err != nil {
		return BlockResult{}, err
	}

	// Aggregate the results and send it back
	// Merge all results per contract
	results := make(chan TraceResult)
	aggregated := make(map[common.Address]*MergedTraceResult)
	done := make(chan struct{})
	go func() {
		for result := range results {
			if existing, exists := aggregated[result.Addr]; exists {
				existing.Bits.Merge(result.Bits)
				existing.CodeOpsCount += result.CodeOpsCount
			} else {
				aggregated[result.Addr] = &MergedTraceResult{
					Bits:         result.Bits,
					CodeOpsCount: result.CodeOpsCount,
				}
			}
		}
		close(done)
	}()

	var workers errgroup.Group
	workers.SetLimit(runtime.NumCPU())
	for _, tx := range trace {
		workers.Go(func() error {
			// fmt.Printf("analyzing tx %d\n", i)
			return a.analyze(&tx, blockNum, results)
		})
	}

	if err := workers.Wait(); err != nil {
		close(results)
		return BlockResult{}, err
	}
	close(results)

	// ---- Uncomment below to debug
	// for i, tx := range trace {
	// 	fmt.Printf("analyzing tx %d\n", i)
	// 	if err := a.analyze(&tx, blockNum, results); err != nil {
	// 		return BlockResult{}, err
	// 	}
	// }
	// close(results)

	// ---- Uncomment below to debug
	// targetTrace := trace[23]
	// fmt.Println(targetTrace.TxHash)
	// if err := a.analyze(&targetTrace, blockNum, results); err != nil {
	// 	return BlockResult{}, err
	// }
	// close(results)

	<-done
	return BlockResult{
		BlockNum: blockNum,
		Results:  aggregated,
	}, nil
}

func (a *Analyzer) analyze(tr *TransactionTrace, blockNum uint64, results chan<- TraceResult) error {
	code, err := a.getCodeFromTx(tr.TxHash, blockNum)
	if err != nil {
		return err
	}

	if len(code.code) == 0 {
		return nil
	}

	index := 0
	res := newTraceResult(code, blockNum)
	lastIndex, err := a.analyzeSteps2(blockNum, &tr.Result, index, res, false, results)
	if err != nil {
		return err
	}

	if lastIndex != len(tr.Result.Steps) {
		return fmt.Errorf("lastIndex mismatch: expected %d, got %d", len(tr.Result.Steps), lastIndex)
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

func (a *Analyzer) analyzeSteps(blockNum uint64, code *Code, trace *InnerResult, depth int, index int, results chan<- TraceResult) (int, error) {
	steps := trace.Steps
	stepsLen := len(steps)
	if stepsLen == 0 {
		return 0, nil
	}

	// Sanity check on the depth
	if depth != steps[index].Depth {
		return 0, fmt.Errorf("(%d) depth mismatch: expected %d, got %d", index, depth, steps[index].Depth)
	}

	result := newTraceResult(code, blockNum)
	for index < stepsLen {
		if index == 4360 { // TODO: remove
			a.log.Info("step 1073")
		}
		step := steps[index]
		fmt.Printf("step %d: pc %d, op %s\n", index, step.PC, step.Op) // TODO: remove

		// We detect that we went back to the previous depth, so this is the end of the current depth
		if step.Depth == depth-1 {
			break
		}

		op := step.Op
		opLen := len(op)

		pc := step.PC
		if pc == uint64(len(code.code)) && op[0] == 'S' {
			// If we've reached the end of the code, this means "STOP" was executed
			// So increment the step index and break to return to the previous depth
			index++
			break
		}

		result.Bits.Set(uint32(pc))

		switch {
		case op == OpPush0:
			// Do nothing
		case opLen > 4 && op[:2] == "PU": // PUSH opcodes
			if err := a.handlePush(result.Bits, &step); err != nil {
				return 0, err
			}
		case opLen > 4 && op[:3] == "COD": // CODEHASH, CODESIZE
			result.CodeOpsCount++
		case opLen == 11 && op[0] == 'E': // EXTCODESIZE, EXTCODEHASH, EXTCODECOPY
			stackTop := step.Stack[len(step.Stack)-1]
			code, err := a.getCode(stackTop, blockNum)
			if err != nil && !trace.Failed {
				return 0, err
			}
			if len(code.code) != 0 {
				extRes := newTraceResult(code, blockNum)
				extRes.CodeOpsCount++
				results <- *extRes
			}
		case opLen == 4 && op[3] == 'L': // CALL
			newIndex, err := a.handleCallOps(step.Stack, depth, index, blockNum, trace, results)
			if err != nil {
				return 0, err
			}
			if newIndex != 0 {
				index = newIndex
				continue
			}
		case opLen == 10 && op[9] == 'L': // STATICCALL
			newIndex, err := a.handleCallOps(step.Stack, depth, index, blockNum, trace, results)
			if err != nil {
				return 0, err
			}
			if newIndex != 0 {
				index = newIndex
				continue
			}
		case opLen == 12 && op[0] == 'D': // DELEGATECALL
			newIndex, err := a.handleCallOps(step.Stack, depth, index, blockNum, trace, results)
			if err != nil {
				return 0, err
			}
			if newIndex != 0 {
				index = newIndex
			}
			continue
		case opLen == 8 && op[2] == 'L': // CALLCODE
			newIndex, err := a.handleCallOps(step.Stack, depth, index, blockNum, trace, results)
			if err != nil {
				return 0, err
			}
			if newIndex != 0 {
				index = newIndex
				continue
			}
		}

		index++
	}

	results <- *result
	return index, nil
}

func (a *Analyzer) analyzeSteps2(blockNum uint64, trace *InnerResult, index int, res *TraceResult, isCreate bool, results chan<- TraceResult) (int, error) {
	steps := trace.Steps
	if len(steps) == 0 {
		return 0, nil
	}

	var prevStep *TraceStep
	if index > 0 {
		prevStep = &steps[index-1]
	}

	for index < len(steps) {
		step := steps[index]
		// if index == 6670 {
		// 	a.log.Info("index 209")
		// }
		// fmt.Printf("step %d: pc %d, op %s depth %d\n", index, step.PC, step.Op, step.Depth) // TODO: remove

		if prevStep != nil {
			if prevStep.Depth > step.Depth {
				if isCreate { // contract creation is done, we are back to the original depth
					isCreate = false
					if step.PC == uint64(res.Bits.Size()) { // Encountered STOP
						prevStep = &step
						index++
						break
					}
					if err := a.handleOps(&step, res, blockNum, trace, results); err != nil {
						return 0, err
					}
					prevStep = &step
					index++
					continue
				}
				// We went back to the previous depth, so this is the end of the current depth
				break
			} else if prevStep.Depth < step.Depth {
				// CREATE or CREATE2
				// Contract creation will increment depth, but we are not interested in analyzing the code
				// So set the isCreate flag to true, continue iterating until we change depth
				if len(prevStep.Op) >= 6 && prevStep.Op[:2] == "CR" {
					isCreate = true
					prevStep = &step
					index++
					continue
				}

				// The current step is a new call, so we need to get the code for the new call and continue
				stackSecond := prevStep.Stack[len(prevStep.Stack)-2]
				code, err := a.getCode(stackSecond, blockNum)
				if err != nil && !trace.Failed {
					return 0, err
				}

				if len(code.code) != 0 {
					res := newTraceResult(code, blockNum)
					if err := a.handleOps(&step, res, blockNum, trace, results); err != nil {
						return 0, err
					}

					newIndex, err := a.analyzeSteps2(blockNum, trace, index+1, res, isCreate, results)
					if err != nil {
						return 0, err
					}
					if newIndex != 0 {
						index = newIndex
						step = steps[index]
					}
				}
			}
		}

		if !isCreate {
			if step.PC == uint64(res.Bits.Size()) { // Encountered STOP
				prevStep = &step
				index++
				break
			}

			if err := a.handleOps(&step, res, blockNum, trace, results); err != nil {
				return 0, err
			}
		}

		prevStep = &step
		index++
	}

	results <- *res
	return index, nil
}

func (a *Analyzer) handleOps(step *TraceStep, res *TraceResult, blockNum uint64, trace *InnerResult, results chan<- TraceResult) error {
	op := step.Op
	opLen := len(op)
	res.Bits.Set(uint32(step.PC))
	switch {
	case op == OpPush0:
		// Do nothing
	case opLen > 4 && op[:2] == "PU": // PUSH opcodes
		if err := a.handlePush(res.Bits, step); err != nil {
			return err
		}
	case opLen > 4 && op[:3] == "COD": // CODEHASH, CODESIZE
		res.CodeOpsCount++
	case opLen == 11 && op[0] == 'E': // EXTCODESIZE, EXTCODEHASH, EXTCODECOPY
		stackTop := step.Stack[len(step.Stack)-1]
		code, err := a.getCode(stackTop, blockNum)
		if err != nil && !trace.Failed {
			return err
		}
		if len(code.code) != 0 {
			extRes := newTraceResult(code, blockNum)
			extRes.CodeOpsCount++
			results <- *extRes
		}
	}
	return nil
}

func (a *Analyzer) handleCallOps(stack []string, depth int, index int, blockNum uint64, trace *InnerResult, results chan<- TraceResult) (int, error) {
	var err error
	var newIndex int

	stackSecond := stack[len(stack)-2]
	code, err := a.getCode(stackSecond, blockNum)
	if err != nil && !trace.Failed {
		return 0, err
	}

	if len(code.code) != 0 {
		newIndex, err = a.analyzeSteps(blockNum, code, trace, depth+1, index+1, results)
		if err != nil {
			return 0, err
		}
	}
	return newIndex, nil
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
