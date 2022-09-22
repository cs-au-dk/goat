package cfg

import (
	"fmt"
	"log"
	"strconv"

	"github.com/cs-au-dk/goat/pkgutil"
	"github.com/cs-au-dk/goat/utils"

	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

type funIO struct {
	in  Node
	out Node
}

// Compute an analysis friendly CFG for the given program SSA IR,
// rewiring control flow for select statements.
// Also takes into account control-flow information, dynamic dispatch,
// as well as calls to defer. Requires points-to information.
func GetCFG(prog *ssa.Program, mains []*ssa.Package, results *pointer.Result) *Cfg {
	cfg.init()
	cfg.fset = prog.Fset

	if main := pkgutil.GetMain(mains); main != nil {
		var initIn, initOut, mainIn Node
		for _, mem := range main.Members {
			switch fun := mem.(type) {
			case *ssa.Function:
				switch {
				case fun.Name() == "init":
					io := getFunCfg(prog, fun, results)
					initIn = io.in
					initOut = io.out
				case fun.Name() == "main":
					mainIn = getFunCfg(prog, fun, results).in
				}
			}
		}

		cfg.addEntry(initIn)
		// Create an edge from the out of `init` to the entry of `main`
		SetSuccessor(initOut, mainIn)
	}

	for _, fun := range pkgutil.TestFunctions(prog) {
		io := getFunCfg(prog, fun, results)
		cfg.addEntry(io.in)
	}

	// fmt.Println("CFG for program:")
	compressCfg()
	// PrintCfg(*cfg)

	return cfg
}

// Convert function definition to CFG.
func getFunCfg(prog *ssa.Program, fun *ssa.Function, results *pointer.Result) funIO {
	// Synthetic node configurations for function exit and entry.
	entryConfig := SynthConfig{
		Function: fun,
		Type:     SynthTypes.FUNCTION_ENTRY,
	}
	exitConfig := SynthConfig{
		Function: fun,
		Type:     SynthTypes.FUNCTION_EXIT,
	}

	// Add (or retrieve if it already exists) the function entry node.
	fwd, new := cfg.addSynthetic(entryConfig)
	// If the function was visited before, retrieve its entry/exit nodes.
	if !new {
		return funIO{in: fwd, out: cfg.GetSynthetic(exitConfig)}
	}

	// Add function exit node.
	bwd, _ := cfg.addSynthetic(exitConfig)

	// Update book-keeping information about functions with new function
	// entry/exit nodes
	fentry := cfg.funs[fun]
	fentry.entry = fwd
	fentry.exit = bwd
	cfg.funs[fun] = fentry
	setDefer(fwd, bwd)
	// TODO: A function without blocks is considered "external" (e. g. a C function).
	// Set the exit node as a direct successor of the entry node and return them.
	if len(fun.Blocks) == 0 {
		opts.OnVerbose(func() {
			fmt.Println("\u001b[33mWARNING\u001b[0m External function ", fun.Name(), " in package ", fun.Pkg.Pkg.Path())
		})
		SetSuccessor(fwd, bwd)
		return funIO{in: fwd, out: bwd}
	}

	// Regular functions process the children blocks in a queue.
	// Loop BLOCK_QUEUE handles one block per iteration.
	// Block queue elements might contain select information for blocks
	type BlockQueue struct {
		Block      *ssa.BasicBlock
		SelectCase ChnSynthetic
	}
	queue := []BlockQueue{{Block: fun.Blocks[0]}}

BLOCK_QUEUE:
	for len(queue) > 0 {
		// Extract the first block in the queue
		blk := queue[0].Block
		// Helper function to add regular synthetic nodes to the current block and function.
		addSynth := func(typ SYNTH_TYPE_ID, suffixes ...string) Node {
			n, _ := cfg.addSynthetic(SynthConfig{
				Type:       typ,
				Block:      blk,
				IdSuffixes: suffixes,
			})
			return n
		}

		// Create synthetic nodes for block entry/exit and their deferred counterparts.
		// Set the current normal/defer flow nodes as the block entry/entry-defer nodes.
		curr := addSynth(SynthTypes.BLOCK_ENTRY)
		currd := addSynth(SynthTypes.BLOCK_ENTRY_DEFER)
		setDefer(curr, currd)

		blkexit := addSynth(SynthTypes.BLOCK_EXIT)
		blkexitd := addSynth(SynthTypes.BLOCK_EXIT_DEFER)
		setDefer(blkexit, blkexitd)

		// More helper functions to avoid code duplication.
		// Handling of path termination nodes (i. e. return, panic).
		// The panicked property states whether the termination is normal,
		// or due to a panic.
		terminatePath := func(i ssa.Instruction, panicked bool) {
			n := cfg.addNode(i)
			SetSuccessor(curr, n)
			if !panicked {
				SetSuccessor(n, blkexit)
			}
			SetSuccessor(blkexit, blkexitd)
			SetSuccessor(blkexitd, currd)
			curr = n
		}
		// Collect outgoing/incoming edges for call instructions.
		getFunIOs := func(i ssa.CallInstruction, suffixes ...string) (funs []funIO) {
			funs = []funIO{}
			call := i.Common()
			if !call.IsInvoke() {
				// If a static, non-builtin, callee is found, leverage it.
				if callee := call.StaticCallee(); callee != nil {
					if len(call.Args) == 1 &&
						utils.IsNamedType(call.Args[0].Type(), "sync", "Cond") &&
						call.Value.Name() == "Wait" &&
						!utils.Opts().SkipSync() {
						waiting, _ := cfg.addSynthetic(SynthConfig{
							Type: SynthTypes.WAITING,
							Insn: i,
							Call: i,
						})
						waking, _ := cfg.addSynthetic(SynthConfig{
							Type: SynthTypes.WAKING,
							Insn: i,
							Call: i,
						})
						SetSuccessor(waiting, waking)

						return append(funs, funIO{waiting, waking})
					}
					return append(funs, getFunCfg(prog, callee, results))
				}
				// Otherwise handle the value of the callee.
				switch f := call.Value.(type) {
				// Builtin functions generate a single node.
				case *ssa.Builtin:
					builtin, _ := cfg.addSynthetic(SynthConfig{
						Type:       SynthTypes.BUILTIN_CALL,
						Call:       i,
						Vals:       []ssa.Value{f},
						Block:      blk,
						IdSuffixes: append([]string{f.Name()}, suffixes...),
					})
					return append(funs, funIO{in: builtin, out: builtin})
				default:
					// Otherwise rely on the points-to information of the function to determine
					// call edges.
					labels := results.Queries[call.Value].PointsTo().Labels()
					for _, label := range labels {
						val, ok := label.Value().(*ssa.Function)
						if !ok {
							log.Fatal("Function points to non-function value")
						}
						funs = append(funs, getFunCfg(prog, val, results))
					}
				}
				return
			}

			// If the call is an interface method invocation, use the
			// points-to set to determine the possible underlying concrete receivers.
			labels := results.Queries[call.Value].PointsTo().Labels()
			for _, label := range labels {
				// For all receivers, check it is an SSA MakeInterface value.
				value, ok := label.Value().(*ssa.MakeInterface)
				if !ok {
					log.Fatal("Dynamic dispatch call on non-interface value:", value)
				}

				// This logic for looking up the targets for invoke calls is equivalent to:
				// func (c *invokeConstraint) solve(...)
				// from golang.org/x/tools/go/pointer/solve.go
				fun := prog.LookupMethod(value.X.Type(), call.Method.Pkg(), call.Method.Name())
				if fun == nil {
					log.Fatalf("%v: No ssa.Function for %v", label, call.Method)
				}

				funs = append(funs, getFunCfg(prog, fun, results))
			}
			return
		}

		// Create node from instruction, and set it
		// as successor to the previous node.
		cont := func(i ssa.Instruction) {
			n := cfg.addNode(i)
			SetSuccessor(curr, n)
			curr = n
		}

		// Instructions to be processed
		var instrs []ssa.Instruction
		// Check if there exists a parent Select case node
		if sel := queue[0].SelectCase; sel != nil {
			// Determine the type of the incoming case node
			switch sel := sel.(type) {
			case *SelectRcv:
				instrs = make([]ssa.Instruction, 0, len(blk.Instrs))
				// Since select case values are obfuscated behind tuple extraction,
				// in the normal SSA, these extraction instructions must be discarded.
				// The Select case synthetic node preserves the assignment value, instead.
				for _, ins := range blk.Instrs {
					switch ins := ins.(type) {
					case *ssa.Extract:
						// Extract the received value, if the receive involves an assignment,
						// and annotate the synthetic node with it. Also extract the
						// `recvOk` field as well, if present.
						if ins.Tuple == sel.Parent.Insn {
							switch ins.Index {
							case 1:
								sel.Ok = ins
							default:
								sel.Val = ins
							}
						} else {
							instrs = append(instrs, ins)
						}
					default:
						instrs = append(instrs, ins)
					}
				}

				// Tie case branch nodes with block entry,
				// and block entry defer node to the deferred select
				SetSuccessor(currd, sel.Parent.DeferLink())
			case *SelectSend:
				SetSuccessor(currd, sel.Parent.DeferLink())
				instrs = blk.Instrs
			case *SelectDefault:
				SetSuccessor(currd, sel.Parent.DeferLink())
				instrs = blk.Instrs
			default:
				// This case indicates a CFG construction bug.
				log.Fatal("Select case predecessor node is not of a Send, Receive or Default")
			}
		} else {
			// For blocks not following a select case branch, tie the
			// block entry and exit nodes (and their deferred variants).
			// Unreachable predecessors not captured by the SSA compiler
			// will have their nodes discarded during compression.
			for _, pred := range blk.Preds {
				predexit, _ := cfg.addSynthetic(SynthConfig{
					Block: pred,
					Type:  SynthTypes.BLOCK_EXIT,
				})
				predexitdef, _ := cfg.addSynthetic(SynthConfig{
					Block: pred,
					Type:  SynthTypes.BLOCK_EXIT_DEFER,
				})
				SetSuccessor(predexit, curr)
				SetSuccessor(currd, predexitdef)
			}
			// The list of block instruction is unchanged
			instrs = blk.Instrs
		}

		queue = queue[1:]

		// Counter for skipping instructions within the same block
		skip := 0
		// Potential second parent (useful for special cases of select).
		var secondParent Node
		for index, ins := range instrs {
			// As long as the skip counter is more than 0, skip instructions
			if skip > 0 {
				skip--
				continue
			}
			// Handle CFG construction based on the type of the SSA instruction.
			switch i := ins.(type) {
			case *ssa.Select:
				// Handle select states without communication cases
				if len(i.States) == 0 {
					// A blocking select without cases will block forever.
					if i.Blocking {
						blocked, _ := cfg.addSynthetic(SynthConfig{
							Type:             SynthTypes.TERMINATE_GORO,
							Block:            blk,
							TerminationCause: GoroTermination.PERMANENTLY_BLOCKED,
						})
						SetSuccessor(curr, blocked)
						continue BLOCK_QUEUE
					} else {
						// A non-blocking select without cases will include
						// a pointless extraction instruction. The other instructions
						// are still in the same block.
						skip = 1
						continue
					}
				}

				// Determine the amount of case branches.
				var opsLen int
				if i.Blocking {
					opsLen = len(i.States)
				} else {
					// Non-blocking selects count the "default" case as a select branch.
					opsLen = len(i.States) + 1
				}
				ops := make([]ChnSynthetic, opsLen)
				// Create a select node, and its deferred counterpart,
				// and connect them to the CFG constructed so far.
				node, _ := cfg.addSynthetic(SynthConfig{
					Type:  SynthTypes.SELECT,
					Insn:  i,
					Block: blk,
				})
				selectNode := node.(*Select)
				selectNode.Insn = i
				selectNode.ops = ops
				selectDefer, _ := cfg.addSynthetic(SynthConfig{
					Type:  SynthTypes.SELECT_DEFER,
					Block: blk,
					Insn:  i,
				})
				setDefer(selectNode, selectDefer)
				SetSuccessor(curr, selectNode)
				SetSuccessor(selectDefer, currd)

				// The result of a select operation is determined via tuple extraction.
				// The 0-th element of the tuple represents the selected case branch index.
				// Find the SSA value corresponding to the index.
				var indexVal *ssa.Extract
				for _, ref := range *i.Referrers() {
					switch ref := ref.(type) {
					case *ssa.Extract:
						if ref.Index == 0 {
							indexVal = ref
							break
						}
					}
				}

				// Non-blocking select instructions with a single case require special handling.
				// Blocking select instructions with a single case are automatically converted by
				// the Go to SSA compiler to a single send/receive instruction.
				if len(i.States) == 1 && !i.Blocking {
					// Determine the SSA value of the guard querying
					// whether the select case index value is 0 at run-time.
					var guard0 ssa.Value
					for _, check0 := range instrs[index+1:] {
						switch check0 := check0.(type) {
						case *ssa.BinOp:
							for _, op := range check0.Operands([]*ssa.Value{}) {
								if *op == indexVal {
									guard0 = check0
									break
								}
							}
						}
					}
					// If no guard was no found, it indicates a select CFG construction bug.
					if guard0 == nil {
						var check0 ssa.Value
						for _, i := range instrs[index+1:] {
							switch i := i.(type) {
							case *ssa.BinOp:
								for _, op := range i.Operands([]*ssa.Value{}) {
									if *op == indexVal {
										check0 = i
										break
									}
								}
							}
						}
						fmt.Println("Function", fun)
						fmt.Println("Index", index)
						fmt.Println("Block:")
						for i, ins := range blk.Instrs {
							switch v := ins.(type) {
							case ssa.Value:
								fmt.Println(i, ":", v.Name(), "=", ins)
							default:
								fmt.Println(i, ":", v)
							}
						}
						fmt.Println("Operands for", check0, "were:")
						switch check0 := check0.(type) {
						case *ssa.BinOp:
							for _, op := range check0.Operands([]*ssa.Value{}) {
								fmt.Println(*op)
							}
						}
						fmt.Println("Was looking for:", indexVal.Name(), "=", indexVal)
						log.Fatal("Check select index 0 not found. Was:", check0)
					}

					// Assume that the current block (containing the select) was not split.
					blockNotSplit := true
					// If the one of the following instructions is an If statement, and its guard
					// involves the selection index, then the current block was split.
					for _, i := range instrs[index+1:] {
						switch if0 := i.(type) {
						case *ssa.If:
							for _, op := range if0.Operands([]*ssa.Value{}) {
								if *op == guard0 {
									blockNotSplit = false
									break
								}
							}
						}
					}
					// If the block was not split, then CFG nodes corresponding
					// to the instructions must be processed within the current block,
					// and appended to its CFG.
					if blockNotSplit {
						// Generate an appropriate synthetic case node,
						// given the direction of the communication operation.
						selectState := i.States[0]
						var node Node
						switch {
						case selectState.Dir == types.SendOnly:
							node, _ = cfg.addSynthetic(SynthConfig{
								Type:         SynthTypes.SELECT_SEND,
								Block:        blk,
								Insn:         i,
								Vals:         []ssa.Value{selectState.Chan, selectState.Send},
								SelectParent: selectNode,
								SelectIndex:  0,
								Pos:          selectState.Pos,
							})
							setPanicCont(node, selectDefer)
						case selectState.Dir == types.RecvOnly:
							// Receive-only select case branches with assignment will
							// always create additional blocks due to the extraction
							// instruction. They should never reach this node creation site.
							node, _ = cfg.addSynthetic(SynthConfig{
								Type:         SynthTypes.SELECT_RCV,
								Block:        blk,
								Insn:         i,
								Vals:         []ssa.Value{selectState.Chan},
								SelectParent: selectNode,
								SelectIndex:  0,
								Pos:          selectState.Pos,
							})
						}
						ops[0] = node.(ChnSynthetic)
						SetSuccessor(selectNode, ops[0])
						// Create a default case node, and set it as a successor to the select node.
						def, _ := cfg.addSynthetic(SynthConfig{
							Type:         SynthTypes.SELECT_DEFAULT,
							Block:        blk,
							Insn:         i,
							SelectParent: selectNode,
							Pos:          selectNode.Pos(),
						})
						curr = ops[0]
						ops[1] = def.(ChnSynthetic)
						SetSuccessor(selectNode, def)
						// Since the select is non-blocking, then the next instruction would also
						// have the default case node of the select as a parent.
						secondParent = def
						// Skip the following two instructions:
						// - the extraction of the selection index from the select value
						// - the guard checking that the selection index equals 0
						skip = 2
						// Proceed to the next instruction, skipping the remaining steps.
						continue
					}
					// If the block was split, then Select CFG construction may proceed normally.
				}

				// The following code involves determining the successor blocks of
				// case of the select statement.
				// Example: given `selectVal` as the SSA tuple value resulting
				// from a select instruction, the SSA code choosing the case
				// indexed as 1 would be expressed as:
				//	index 						= extract #0 select
				// 	selectCase1Guard 	= index == 1
				//  if selectCase1Guard 1 else 2
				// Where 1 represents the case successor block, and 2 represents
				// the block leading to a different choice.

				// For every SSA instruction using the selection index SSA value as an operand:
				for _, ref := range *indexVal.Referrers() {
					switch ref := ref.(type) {
					case *ssa.BinOp:
						if ref.Op == token.EQL {
							var selIndex int
							// If the instruction is a binary equality, extract the second
							// operand of the equality. Since these represent the select cases control
							// flow, they are statically known and should be represented as integer constants.
							switch ind := ref.Y.(type) {
							case *ssa.Const:
								if ind.Value.Kind() == constant.Int {
									selIndexRaw, _ := constant.Int64Val(ind.Value)
									selIndex = (int)(selIndexRaw)
								} else {
									log.Fatal("Select index not integer")
								}
							default:
								log.Fatal("Select index not constant")
							}
							// Find all referrers of the case selection guard.
							refs := ref.Referrers()
							if refs != nil && len(*refs) == 1 {
								// The only referrer should be an if statement determining
								// the successor block in the control flow.
								ifst, ok := (*refs)[0].(*ssa.If)
								if !ok {
									log.Fatal("Select index referrer not an If instruction", selectNode, selIndex)
								}
								// Extract the if statement parent block `then` successor.
								thenBlock := ifst.Block().Succs[0]
								selectState := i.States[selIndex]
								// Construct a synthetic case node, based on the case
								// communication operation.
								var selectStateNode ChnSynthetic
								if selectState.Dir == types.SendOnly {
									node, _ := cfg.addSynthetic(SynthConfig{
										Type:         SynthTypes.SELECT_SEND,
										Block:        blk,
										SelectIndex:  selIndex,
										Insn:         i,
										SelectParent: selectNode,
										Vals:         []ssa.Value{selectState.Chan, selectState.Send},
										Pos:          selectState.Pos,
									})
									selectStateNode = node.(ChnSynthetic)
									setPanicCont(node, selectDefer)
								} else {
									node, _ := cfg.addSynthetic(SynthConfig{
										Type:         SynthTypes.SELECT_RCV,
										Block:        blk,
										SelectIndex:  selIndex,
										Insn:         i,
										SelectParent: selectNode,
										Vals:         []ssa.Value{selectState.Chan},
										Pos:          selectState.Pos,
									})
									selectStateNode = node.(ChnSynthetic)
								}

								ops[selIndex] = selectStateNode
								SetSuccessor(selectNode, selectStateNode)

								// Make sure we get a branching statement between
								// the selectStateNode and the thenBlock entry.
								cpIfStmt := *ifst
								cpIfStmt.Cond = &ssa.Const{Value: constant.MakeBool(true)}
								ifNode := cfg.addNode(&cpIfStmt)
								SetSuccessor(selectStateNode, ifNode)

								// Connect the successor block entry with the select case branch.
								caseBlock, new := cfg.addSynthetic(SynthConfig{
									Type:  SynthTypes.BLOCK_ENTRY,
									Block: thenBlock,
								})
								SetSuccessor(ifNode, caseBlock)
								// If the block was not previously explored, add it to the queue.
								if new {
									caseBlockDefer, _ := cfg.addSynthetic(SynthConfig{
										Type:  SynthTypes.BLOCK_ENTRY_DEFER,
										Block: thenBlock,
									})
									setDefer(caseBlock, caseBlockDefer)
									queue = append(queue, BlockQueue{
										Block:      thenBlock,
										SelectCase: selectStateNode,
									})
								} else {
									SetSuccessor(caseBlock.DeferLink(), selectDefer)
								}
								// If the select is non-blocking, create a default synthetic node,
								// and add the second successor block to the queue, if it was not
								// previously explored.
								if !i.Blocking && selIndex == opsLen-2 {
									elseBlock := ifst.Block().Succs[1]
									defPos := selectNode.Pos()
									if pos := elseBlock.Instrs[0].Pos(); pos.IsValid() {
										defPos = pos
									}
									defaultNode, _ := cfg.addSynthetic(SynthConfig{
										Type:         SynthTypes.SELECT_DEFAULT,
										SelectParent: selectNode,
										Insn:         i,
										Block:        blk,
										Pos:          defPos,
									})
									ops[selIndex+1] = defaultNode.(ChnSynthetic)
									SetSuccessor(selectNode, ops[selIndex+1])

									// Make sure we get a branching statement between
									// the defaultNode and the elseBlock entry.
									cpIfStmt := *ifst
									cpIfStmt.Cond = &ssa.Const{Value: constant.MakeBool(false)}
									ifNode := cfg.addNode(&cpIfStmt)
									SetSuccessor(ops[selIndex+1], ifNode)

									defaultEntry, new := cfg.addSynthetic(SynthConfig{
										Type:  SynthTypes.BLOCK_ENTRY,
										Block: elseBlock,
									})
									SetSuccessor(ifNode, defaultEntry)
									if new {
										defaultEntryDefer, _ := cfg.addSynthetic(SynthConfig{
											Type:  SynthTypes.BLOCK_ENTRY_DEFER,
											Block: elseBlock,
										})
										setDefer(defaultEntry, defaultEntryDefer)
										queue = append(queue, BlockQueue{
											Block:      elseBlock,
											SelectCase: defaultNode.(ChnSynthetic),
										})
									} else {
										SetSuccessor(defaultEntry.DeferLink(), selectDefer)
									}
								}
							} else {
								// There should strictly one or no referrers.
								if len(*refs) != 0 {
									fmt.Println("More then one referrer of the case selection guard found:", selectNode)
									log.Fatal("Index:", selIndex)
								}
								// If no referrers are found, then the only (TODO: find more) scenario is a
								// non-blocking select statement with more than one case, where the last and default
								// cases have no bodies. No check is performed on the guard, instead
								// jumping unconditionally to the successor block.
								// A blocking select statement should never lead to this situation:
								// there is always a Panic block in case the select index does not match
								// any of the case indexes.
								succBlock := ref.Block().Succs[0]
								selectState := i.States[selIndex]
								var selectStateNode ChnSynthetic
								if selectState.Dir == types.SendOnly {
									node, _ := cfg.addSynthetic(SynthConfig{
										Type:         SynthTypes.SELECT_SEND,
										Block:        blk,
										SelectIndex:  selIndex,
										Insn:         i,
										SelectParent: selectNode,
										Vals:         []ssa.Value{selectState.Chan, selectState.Send},
										Pos:          selectState.Pos,
									})
									selectStateNode = node.(ChnSynthetic)
									setPanicCont(node, selectDefer)
								} else if selectState.Dir == types.RecvOnly {
									// No need to preserve the received value. The Go compiler will
									// disallow not using its declared name. This enforced usage would
									// result in a separate succesor block for the receive case, thus leading
									// to the existence of a referrer to the case index guard.
									node, _ := cfg.addSynthetic(SynthConfig{
										Type:         SynthTypes.SELECT_RCV,
										Block:        blk,
										Insn:         i,
										SelectIndex:  selIndex,
										SelectParent: selectNode,
										Vals:         []ssa.Value{selectState.Chan},
										Pos:          selectState.Pos,
									})
									selectStateNode = node.(ChnSynthetic)
								}
								ops[selIndex] = selectStateNode
								SetSuccessor(selectNode, selectStateNode)

								// Create a default node and connect it to the same successor
								// block.
								defPos := selectNode.Pos()
								if pos := succBlock.Instrs[0].Pos(); pos.IsValid() {
									defPos = pos
								}
								defaultNode, _ := cfg.addSynthetic(SynthConfig{
									Type:         SynthTypes.SELECT_DEFAULT,
									SelectParent: selectNode,
									Insn:         i,
									Block:        blk,
									Pos:          defPos,
								})
								ops[selIndex+1] = defaultNode.(ChnSynthetic)
								SetSuccessor(selectNode, defaultNode)

								// Make sure we get a branching statement between
								// the selectStateNode and the caseBlock entry.
								jump := ref.Block().Instrs[len(ref.Block().Instrs)-1].(*ssa.Jump)
								jumpNode := cfg.addNode(jump)
								SetSuccessor(selectStateNode, jumpNode)
								SetSuccessor(defaultNode, jumpNode)

								caseBlock, new := cfg.addSynthetic(SynthConfig{
									Type:  SynthTypes.BLOCK_ENTRY,
									Block: succBlock,
								})
								SetSuccessor(jumpNode, caseBlock)
								if new {
									caseBlockDefer, _ := cfg.addSynthetic(SynthConfig{
										Type:  SynthTypes.BLOCK_ENTRY_DEFER,
										Block: succBlock,
									})
									setDefer(caseBlock, caseBlockDefer)
									queue = append(queue, BlockQueue{
										Block:      succBlock,
										SelectCase: selectStateNode,
									})
								} else {
									SetSuccessor(caseBlock.DeferLink(), selectDefer)
								}
							}
						}
					}
				}
				// A select statement and its cases should act as synthetic block
				// terminators.
				continue BLOCK_QUEUE
			case *ssa.Panic:
				terminatePath(i, true)
				if secondParent != nil {
					SetSuccessor(secondParent, curr)
				}
				setPanicCont(curr, blkexit)
				continue BLOCK_QUEUE
			case *ssa.Return:
				terminatePath(i, false)
				if secondParent != nil {
					SetSuccessor(secondParent, curr)
				}
				continue BLOCK_QUEUE
			case *ssa.Defer:
				strIndex := strconv.Itoa(index)
				def := cfg.addNode(i)
				defdpre := addSynth(SynthTypes.DEFER_CALL, strIndex)
				defdpost := addSynth(SynthTypes.POST_DEFER_CALL, strIndex)
				funIOs := getFunIOs(i, strIndex)
				SetSuccessor(defdpost, currd)
				setPanicCont(defdpost, currd)
				switch len(funIOs) {
				case 0:
					// If the points-to set is empty, then it is a guaranteed
					// nil dereference panic. The post-defer call will be set as the panic
					// continuation, but there is no regular successor.
				case 1:
					if builtin, ok := funIOs[0].in.(*BuiltinCall); ok {
						// It's useful to have the post-defer node so we wire it up
						// but discard the pre-defer call.
						SetSuccessor(builtin, defdpost)
						setPanicCont(builtin, defdpost)
						setCall(builtin, defdpost)
						setDefer(def, builtin)
						currd = builtin
						goto WIRE_DEFER_INSTRUCTION
					}
					// Reachable only if the single element in the points-to set
					// is not a builtin call. Handle successor wiring from pre-defer call
					// node to function entry, and from function exit to post-defer call
					// node as normal.
					fallthrough
				default:
					for _, cfafun := range funIOs {
						SetSuccessor(defdpre, cfafun.in)
						SetSuccessor(cfafun.out, defdpost)
					}
				}
				setCall(defdpre, defdpost)
				setDefer(def, defdpre)
				setPanicCont(defdpre, defdpost)
				currd = defdpre
			WIRE_DEFER_INSTRUCTION:
				SetSuccessor(curr, def)
				curr = def
			case *ssa.Go:
				gon := cfg.addNode(i)
				setPanicCont(gon, currd)
				SetSuccessor(curr, gon)
				curr = gon
				funIOs := getFunIOs(i, strconv.Itoa(index))
				isBuiltin := false

				switch len(funIOs) {
				case 0:
					// If the points-to set of the function is empty, a guaranteed
					// nil dereference panic means the current block is guaranteed to terminate here.
					continue BLOCK_QUEUE
				case 1:
					// If the points-to set is a singleton, check whether it is a
					// built-in call. If so, wire it up with a synthetic function exit.
					if builtin, ok := funIOs[0].in.(*BuiltinCall); ok {
						exit, _ := cfg.addSynthetic(SynthConfig{
							Type:       SynthTypes.FUNCTION_EXIT,
							Insn:       i,
							IdSuffixes: []string{builtin.builtin.Name(), strconv.Itoa(index)},
						})

						SetSuccessor(builtin, exit)
						// If the built-in might panic, the synthetic exit is also its
						// continuation node.
						setPanicCont(builtin, exit)
						isBuiltin = true
					}
				}

				for _, cfafun := range funIOs {
					in := cfafun.in
					// If the spawn is a concurrent operation on a non-builtin
					if callCommonIsConcurrent(i.Call) && !isBuiltin {
						exit, _ := cfg.addSynthetic(SynthConfig{
							Type:       SynthTypes.FUNCTION_EXIT,
							Insn:       i,
							IdSuffixes: []string{in.String(), strconv.Itoa(index)},
						})
						in, _ = cfg.addSynthetic(SynthConfig{
							Type:       SynthTypes.API_CONC_BUILTIN,
							Call:       i,
							Insn:       i,
							Function:   cfafun.in.Function(),
							IdSuffixes: []string{in.String(), strconv.Itoa(index)},
						})
						SetSuccessor(in, exit)
						setPanicCont(in, exit)
						setCall(in, exit)
					}
					setSpawn(gon, in)
				}
			case *ssa.Call:
				strIndex := strconv.Itoa(index)
				call := cfg.addNode(i)
				funIOs := getFunIOs(i, strIndex)
				setPanicCont(call, currd)
				switch len(funIOs) {
				case 0:
					// If the points-to set of the function is empty, only a guaranteed
					// nil dereference panic will occur.
					SetSuccessor(curr, call)
					// Current block is guaranteed to terminate here.
					continue BLOCK_QUEUE
				case 1:
					// If the points-to set is a singleton, check whether it is a
					// built-in call. If so, treat it as a single instruction node.
					switch builtin := funIOs[0].in.(type) {
					case *BuiltinCall:
						SetSuccessor(curr, builtin)
						setPanicCont(builtin, currd)
						curr = builtin
						continue
					}
					// Reachable only if the single element in the points-to set
					// is not a builtin call.
					fallthrough
				default:
					callpost := addSynth(SynthTypes.POST_CALL, strIndex)
					setCall(call, callpost)
					setPanicCont(callpost, currd)
					// For every function that might alias the one at the current
					// call, add an outgoing edge to its entry, and tie the exit node
					// to the post-call node
					for _, cfafun := range funIOs {
						SetSuccessor(call, cfafun.in)
						SetSuccessor(cfafun.out, callpost)
					}
					SetSuccessor(curr, call)
					curr = callpost
				}
			case *ssa.Alloc:
				cont(i)
			case *ssa.ChangeType:
				cont(i)
			case *ssa.Convert:
				cont(i)
			case *ssa.DebugRef:
				// Skip
				continue
			case *ssa.Extract:
				cont(i)
			case *ssa.Field:
				cont(i)
			case *ssa.If:
				cont(i)
			case *ssa.Jump:
				cont(i)
			case *ssa.MakeClosure:
				cont(i)
			case *ssa.MakeInterface:
				cont(i)
			case *ssa.MakeMap:
				cont(i)
			case *ssa.Range:
				cont(i)
			case *ssa.RunDefers:
				// Skip
				continue
			case *ssa.TypeAssert:
				cont(i)
				if !i.CommaOk {
					setPanicCont(curr, currd)
				}
			default:
				cont(i)
				setPanicCont(curr, currd)
			}

			if secondParent != nil {
				if pcall, isPostCall := curr.(*PostCall); isPostCall {
					// Make the secondParent use the call node as a successor
					// instead of the post-call node.
					SetSuccessor(secondParent, pcall.CallRelationNode())
				} else {
					SetSuccessor(secondParent, curr)
				}

				secondParent = nil
			}
		}

		SetSuccessor(curr, blkexit)
		SetSuccessor(blkexitd, currd)

		// If there are no successors, connect the block exit and exit defer nodes.
		if len(blk.Succs) == 0 {
			SetSuccessor(blkexit, blkexitd)
		}

		// Connect the CFG with all successor blocks, and
		// add them to the queue, if not previously explored.
		for _, succ := range blk.Succs {
			succentry, new := cfg.addSynthetic(SynthConfig{
				Type:  SynthTypes.BLOCK_ENTRY,
				Block: succ,
			})
			succentryd, _ := cfg.addSynthetic(SynthConfig{
				Type:  SynthTypes.BLOCK_ENTRY_DEFER,
				Block: succ,
			})
			if new {
				setDefer(succentry, succentryd)
				queue = append(queue, BlockQueue{Block: succ})
			}
			SetSuccessor(blkexit, succentry)
			SetSuccessor(succentryd, blkexitd)
		}
	}

	// Connect function exit and entry nodes to entry block
	blk0 := cfg.GetSynthetic(SynthConfig{
		Type:  SynthTypes.BLOCK_ENTRY,
		Block: fun.Blocks[0],
	})
	blk0d := blk0.DeferLink()
	SetSuccessor(cfg.GetSynthetic(entryConfig), blk0)
	SetSuccessor(blk0d, cfg.GetSynthetic(exitConfig))

	// Retrieve function entry and exit nodes.
	return funIO{in: fwd, out: bwd}
}

func (g *Cfg) GetAllConcurrencyOps() (res map[ssa.Instruction]struct{}) {
	res = make(map[ssa.Instruction]struct{})
	visited := make(map[Node]struct{})

	var getAllConcurrencyOps func(Node)
	getAllConcurrencyOps = func(n Node) {
		if _, ok := visited[n]; ok {
			return
		}

		visited[n] = struct{}{}
		if pkgutil.CheckInGoroot(n.Function()) {
			return
		}

		switch n := n.(type) {
		case *SSANode:
			if n.IsCommunicationNode() {
				res[n.insn] = struct{}{}
				switch n.Instruction().(type) {
				case *ssa.Call:
					getAllConcurrencyOps(n.CallRelationNode())
					return
				}
			}
		case *Select:
			res[n.Insn] = struct{}{}
		case *BuiltinCall:
			if n.IsCommunicationNode() {
				res[n.Call] = struct{}{}
			}
		case *DeferCall:
			if n.IsCommunicationNode() {
				res[n.Instruction()] = struct{}{}
				getAllConcurrencyOps(n.CallRelationNode())
				return
			}
		}

		for s := range n.Successors() {
			getAllConcurrencyOps(s)
		}
		for s := range n.Spawns() {
			getAllConcurrencyOps(s)
		}
	}

	for n := range g.entries {
		getAllConcurrencyOps(n)
	}

	return
}

func (g *Cfg) GetAllChans() (res map[ssa.Instruction]struct{}) {
	res = make(map[ssa.Instruction]struct{})
	visited := make(map[Node]struct{})

	var getAllChans func(Node)
	getAllChans = func(n Node) {
		if _, ok := visited[n]; ok {
			return
		}

		visited[n] = struct{}{}
		if pkgutil.CheckInGoroot(n.Function()) {
			return
		}

		switch n := n.(type) {
		case *SSANode:
			switch n.Instruction().(type) {
			case *ssa.MakeChan:
				res[n.insn] = struct{}{}
			}
		}

		for s := range n.Successors() {
			getAllChans(s)
		}
		for s := range n.Spawns() {
			getAllChans(s)
		}
	}

	for n := range g.entries {
		getAllChans(n)
	}

	return
}

func (g *Cfg) GetAllGos() (res map[ssa.Instruction]struct{}) {
	res = make(map[ssa.Instruction]struct{})
	visited := make(map[Node]struct{})

	var getAllGos func(Node)
	getAllGos = func(n Node) {
		if _, ok := visited[n]; ok {
			return
		}

		visited[n] = struct{}{}
		if pkgutil.CheckInGoroot(n.Function()) {
			return
		}

		switch n := n.(type) {
		case *SSANode:
			switch n.Instruction().(type) {
			case *ssa.Go:
				res[n.insn] = struct{}{}
			case ssa.CallInstruction:
				if n.IsCommunicationNode() {
					getAllGos(n.CallRelationNode())
					return
				}
			}
		case *DeferCall:
			if n.IsCommunicationNode() {
				getAllGos(n.CallRelationNode())
				return
			}
		}

		for s := range n.Successors() {
			getAllGos(s)
		}
		for s := range n.Spawns() {
			getAllGos(s)
		}
	}

	for n := range g.entries {
		getAllGos(n)
	}

	return
}
