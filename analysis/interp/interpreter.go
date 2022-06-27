package interp

// type goroLabel = defs.Goro
// type CtrlLoc = cfg.Node // TODO
// type Node = map[goroLabel]CtrlLoc
// type Var = int         // TODO
// type Value interface{} // TODO

// type Pointer struct {
// 	goro  goroLabel
// 	label Var // Actual location
// }

// /* Deref:
// var v Value
// ...
// p := v.(*Pointer)
// if p.goro = -1 {
// 	// look in heap
// } else {
// 	// look in env
// }
// */

// type Frame struct {
// 	env          *immutable.Map // Var -> Value
// 	instrToLabel *immutable.Map // ssa.Value -> Var
// }

// func (f Frame) lookupValue(v ssa.Value) Value {
// 	label, found := f.instrToLabel.Get(v)

// 	if !found {
// 		log.Fatalln("Value (ssa.Value) not found!")
// 	}

// 	value, found := f.env.Get(label)

// 	if !found {
// 		log.Fatalln("Value not found!")
// 	}

// 	return value
// }

// type State struct {
// 	globals *immutable.Map // ssa.Global -> Pointer

// 	envs *immutable.Map // GoroLabel -> Frame
// }

// func (s State) getFrame(goro goroLabel) Frame {
// 	frameItf, found := s.envs.Get(goro)
// 	if !found {
// 		log.Fatalf("Frame for GoroLabel: %s doesn't exist in state?\n", goro)
// 	}

// 	frame, ok := frameItf.(Frame)
// 	if !ok {
// 		log.Fatalln("Unexpected non-Frame struct in state.envs?")
// 	}

// 	return frame
// }

// /* Since Go's builtin maps are not hashable, we cannot use them as keys in other maps.
//    Since Node is a map and we want to use it as a key in another map, we implement
//    the other map from scratch as an associative list.
//    reflect.DeepEqual is used to determine equality between nodes.
// type stateMap interface {
// 	lookup(Node) State
// 	insert(Node, State)
// }
// type entry struct {
// 	key   Node
// 	value State
// }
// type stateList []entry
// func (sl stateList) findEntry(key Node) *entry {
// 	for _, e := range sl {
// 		if reflect.DeepEqual(e.key, key) {
// 			return &e
// 		}
// 	}
// 	return nil
// }
// func (sl stateList) lookup(key Node) State {
// 	if e := sl.findEntry(key); e != nil {
// 		return e.value
// 	}
// 	return nil
// }
// func (sl *stateList) insert(key Node, value State) {
// 	if e := sl.findEntry(key); e != nil {
// 		e.value = value
// 	} else {
// 		(*sl) = append(*sl, entry{key, value})
// 	}
// }
// */

// type PointerHasher = utils.PointerHasher

// func allocHeap(state *State) (p Pointer) {
// 	p.goro = "0"
// 	frame := state.getFrame("0")

// 	p.label = frame.env.Len()
// 	frame.env = frame.env.Set(p.label, 0)

// 	state.envs = state.envs.Set(p.goro, frame)

// 	return
// }

// func Analyze(prog *ssa.Program, G *cfg.Cfg) {
// 	prog_entries := G.GetEntries()

// 	if len(prog_entries) != 1 {
// 		log.Fatalln("Cannot handle program with multiple entries")
// 	}

// 	pos := Node{"0": prog_entries[0]}
// 	init_frame := Frame{
// 		env:          immutable.NewMap(nil),
// 		instrToLabel: immutable.NewMap(&PointerHasher{}),
// 	}

// 	state := State{
// 		globals: immutable.NewMap(&PointerHasher{}),
// 		envs:    immutable.NewMap(nil).Set(0, init_frame),
// 	}

// 	for _, pkg := range prog.AllPackages() {
// 		for _, member := range pkg.Members {
// 			if glob, ok := member.(*ssa.Global); ok {
// 				state.globals = state.globals.Set(glob, allocHeap(&state))
// 			}
// 		}
// 	}

// LOOP:
// 	for {
// 		log.Printf("at %v with %v\n", pos, state)

// 		// TODO: Handle more threads
// 		if len(pos) != 1 {
// 			log.Fatalln("More threads TODO")
// 		}

// 		node := pos["0"]
// 		//utils.PrintSSAFun(node.Function())
// 		succs := node.Successors()

// 		switch cnode := node.(type) {
// 		case cfg.AnySynthetic:
// 			//log.Println(cnode.String())

// 		case *cfg.SSANode:

// 			ins := cnode.Instruction()
// 			log.Printf("SSA node type: %T\n", ins)

// 			switch cins := ins.(type) {
// 			case *ssa.If:
// 				frame := state.getFrame("0")
// 				condV := frame.lookupValue(cins.Cond)

// 				ops := []*ssa.Value{}
// 				ops = cins.Operands(ops)

// 				block := cins.Block()
// 				bsuccs := block.Succs

// 				var succBlock *ssa.BasicBlock
// 				if condV == 0 {
// 					succBlock = bsuccs[1]
// 				} else {
// 					succBlock = bsuccs[0]
// 				}

// 				succNode := G.GetSynthetic(cfg.SynthConfig{
// 					Type:     cfg.SynthTypes.BLOCK_ENTRY,
// 					Block:    succBlock,
// 					Function: cins.Parent(),
// 				})

// 				if succNode == nil {
// 					log.Fatalln("Unable to find successor")
// 				}

// 				pos["0"] = succNode
// 				continue LOOP

// 			case *ssa.Jump, *ssa.Return:
// 				// Continue using CFG successor

// 			case *ssa.Store:
// 				ptrAsValue := evalValue(state, cins.Addr)
// 				ptr, ok := ptrAsValue.(Pointer)
// 				if !ok {
// 					log.Fatalf("Expected Pointer in ssa.Store but got %T (%v)\n", ptrAsValue, ptrAsValue)
// 				}

// 				frame := state.getFrame(ptr.goro)
// 				frame.env = frame.env.Set(ptr.label, evalValue(state, cins.Val))
// 				state.envs.Set(ptr.goro, frame)

// 			case ssa.Instruction:
// 				val := evalValue(state, cins.(ssa.Value))

// 				frame := state.getFrame("0")

// 				label := frame.env.Len()
// 				frame.instrToLabel = frame.instrToLabel.Set(cins, label)
// 				frame.env = frame.env.Set(label, val)

// 				state.envs = state.envs.Set(0, frame)

// 			default:
// 				log.Fatalln("???")
// 			}

// 		default:
// 			log.Fatalln("New Node???")
// 		}

// 		if len(succs) != 1 {
// 			log.Println(succs)
// 			log.Fatalln("Oh no")
// 		}

// 		for next := range succs {
// 			pos["0"] = next
// 			break
// 		}

// 	}
// }

// func evalValue(state State, value ssa.Value) Value {
// 	switch ins := value.(type) {
// 	case *ssa.Const:

// 		if t, ok := ins.Type().Underlying().(*types.Basic); ok {
// 			switch t.Kind() {
// 			case types.Bool:
// 				return constant.BoolVal(ins.Value)
// 			}
// 		}

// 		log.Fatalf("Don't know how to evaluate const %+v\n", ins)

// 	case *ssa.Global:
// 		if glob, ok := state.globals.Get(value); ok {
// 			return glob
// 		}

// 		log.Fatalln("Global doesn't exist")

// 	case *ssa.UnOp:
// 		inner := evalValue(state, ins.X)

// 		switch ins.Op {
// 		case token.MUL:
// 			if p, ok := inner.(Pointer); ok {
// 				val, found := state.getFrame(p.goro).env.Get(p.label)

// 				if !found {
// 					log.Fatalf("Could not find loc %d in goro %s's env\n", p.label, p.goro)
// 				}

// 				return val
// 			}

// 			log.Fatal("Value to deref is not pointer")
// 			//log.Fatalln("???")
// 		}

// 	case *ssa.Call:
// 		// ins.Call

// 	default:
// 		log.Fatalf("Don't know how to eval %+v %T\n", value, value)
// 	}

// 	return 0
// }
