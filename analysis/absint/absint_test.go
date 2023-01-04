package absint

import (
	"runtime/debug"
	"strings"
	"testing"

	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	loc "github.com/cs-au-dk/goat/analysis/location"
	"github.com/cs-au-dk/goat/testutil"

	"golang.org/x/tools/go/ssa"
)

func loadProgram(t *testing.T, content string) AnalysisCtxt {
	loadRes := testutil.LoadPackageFromSource(t, "testpackage", content)

	if opts.Visualize() {
		loadRes.Cfg.Visualize(&loadRes.Pointer.Result)
	}

	ctxt := PrepareAI().FunctionByName("tmain", false)(loadRes)
	//ctxt.AnalyzeCallsWithoutConcurrencyPrimitives = true
	ctxt.setFragmentPredicate(false, true)
	return ctxt
}

type absIntTestFunc = func(*testing.T, AnalysisCtxt)
type absIntTest struct {
	name    string
	content string
	doTest  absIntTestFunc
}

type spoofedGoro struct{}

func (spoofedGoro) String() string { return "spoofed-goroutine" }
func (spoofedGoro) Hash() uint32   { return 42 }

func TestAbstractInterpretation(t *testing.T) {
	chLoc := loc.LocalLocation{
		Goro:     spoofedGoro{},
		Name:     "spoofed-location",
		DeclLine: -1,
	}

	// Check if the testpackage has a bool variable named ubool. If so, give it a top value.
	injectUBool := func(ctxt *AnalysisCtxt) {
		if glob := ctxt.InitConf.Main().CtrLoc().Root().Package().Var("ubool"); glob != nil {
			state := ctxt.InitState
			ctxt.InitState = state.UpdateMemory(state.Memory().Update(
				loc.GlobalLocation{Site: glob},
				Elements().AbstractBasic(false).ToTop(),
			))
		}
	}

	progressTest := func(expectFinish bool) absIntTestFunc {
		return func(t *testing.T, ctxt AnalysisCtxt) {
			injectUBool(&ctxt)
			conf, _ := CoarseProgress(ctxt)
			node := conf.GetUnsafe(ctxt.InitConf.Main()).Node()
			if _, ok := node.(*cfg.TerminateGoro); expectFinish && !ok {
				t.Error("The main thread did not reach its exit:", node)
			}
		}
	}

	checkNoPanic := progressTest(true)

	// Don't know if this is smart...
	testFactoryWMem := func(
		makeTest func(*testing.T, defs.Goro, L.Memory) (
			L.Memory,
			func(L.Memory, L.AbstractValue),
		),
	) absIntTestFunc {
		return func(t *testing.T, ctxt AnalysisCtxt) {
			g := ctxt.InitConf.Main()
			mem, testFunc := makeTest(t, g, ctxt.InitState.Memory())
			ctxt.InitState = ctxt.InitState.UpdateMemory(mem)
			injectUBool(&ctxt)

			succs := ctxt.InitConf.GetTransitions(ctxt, ctxt.InitState)
			// Filter out panicked successors
			for key, succ := range succs {
				if succ.Configuration().IsPanicked() {
					// TODO: Let caller control whether panicking is allowed?
					delete(succs, key)
				}
			}

			if len(succs) != 1 {
				t.Fatal("Not exactly one possible successor?", succs)
			}

			for _, succ := range succs {
				cl := succ.Configuration().GetUnsafe(g)
				if _, ok := cl.Node().(*cfg.TerminateGoro); !ok {
					t.Fatal("The main thread did not reach its exit:", cl)
				}

				state := succ.State
				retLoc := loc.ReturnLocation(g, g.CtrLoc().Root())
				retVal, found := state.Memory().Get(retLoc)
				if !found {
					t.Fatal("No value set at return location?")
				}

				testFunc(state.Memory(), retVal)
			}
		}
	}

	testFactory := func(
		makeTest func(*testing.T, defs.Goro, L.Memory) (
			L.Memory,
			func(L.AbstractValue),
		),
	) absIntTestFunc {
		return testFactoryWMem(func(t *testing.T, g defs.Goro, mem L.Memory) (
			L.Memory,
			func(L.Memory, L.AbstractValue),
		) {
			updMem, tFun := makeTest(t, g, mem)
			return updMem, func(_ L.Memory, retVal L.AbstractValue) {
				tFun(retVal)
			}
		})
	}

	paramEqualsReturnValue := testFactory(func(t *testing.T, entry defs.Goro, mem L.Memory) (
		L.Memory,
		func(L.AbstractValue),
	) {
		mem = mem.Update(
			chLoc,
			makeChannelValue(
				Lattices().FlatInt().Top().Flat(),
				true,
				0,
			),
		)

		f := entry.CtrLoc().Root()
		globLoc := loc.GlobalLocation{Site: f.Package().Var("ch")}
		paramVal := Elements().AbstractPointerV(chLoc)
		mem = mem.Update(globLoc, paramVal)

		return mem, func(retVal L.AbstractValue) {
			if !retVal.Eq(paramVal) {
				t.Errorf("Expected %v to equal %v\n", retVal, paramVal)
			}
		}
	})

	simpleChannelTest := func(expectSingleton, expectNil bool) absIntTestFunc {
		return testFactoryWMem(func(t *testing.T, entry defs.Goro, mem L.Memory) (
			L.Memory,
			func(L.Memory, L.AbstractValue),
		) {
			return mem, func(mem_ L.Memory, val L.AbstractValue) {
				mem := L.MemOps(mem_)

				expectedSize := 1
				if expectNil {
					expectedSize += 1
				}

				ptsto := val.PointerValue()
				if ptsto.Empty() {
					t.Fatal("Points-to set for", val, "is empty")
				} else if ptsto.Size() != expectedSize {
					t.Fatal("Points-to set for", val, "contains too many entries:", ptsto, expectedSize)
				}

				if expectNil && !ptsto.Contains(loc.NilLocation{}) {
					t.Error("Expected", ptsto, "to contain nil")
				}

				ptsto = ptsto.Remove(loc.NilLocation{})

				if strongOk := mem.CanStrongUpdate(ptsto); expectSingleton != strongOk {
					t.Log(mem.Memory(), ptsto)
					t.Error("Expected singleton:", expectSingleton, "but was actually:", strongOk)
				}

				allocSite := ptsto.Entries()[0]
				chVal, found := mem.Get(allocSite)
				if !found {
					t.Fatal("Memory did not contain a value for", allocSite)
				}

				chInfo := chVal.ChanValue()
				status := chInfo.Status()
				if status.IsTop() || status.IsBot() {
					t.Fatal("Channel status for", allocSite, "is not a boolean:", status)
				}

				if !status.Value().(bool) {
					t.Fatal("Channel is not open")
				}

				fbuffer := chInfo.BufferFlat()
				if fbuffer.IsTop() || fbuffer.IsBot() {
					t.Fatal("Buffer value for", allocSite, "is not an integer:", fbuffer)
				}

				bufVal := fbuffer.(L.FlatIntElement).IValue()
				if bufVal != 0 {
					t.Fatal("Buffer value is non-zero:", bufVal)
				}
			}
		})
	}

	arrTopTest := testFactory(func(t *testing.T, _ defs.Goro, mem L.Memory) (L.Memory, func(L.AbstractValue)) {
		return mem, func(val L.AbstractValue) {
			if val.IsStruct() {
				// Unwrap array
				val = val.StructValue().Get(loc.AINDEX).AbstractValue()
			}

			if !val.IsBasic() {
				t.Fatal("Expected", val, "to be a basic value.")
			} else if !val.BasicValue().IsTop() {
				t.Error("Expected", val, "to be ⊤.")
			}
		}
	})

	// Checks that the result of silent transitions contains a configuration where g is panicked
	checkMayPanic := func(t *testing.T, ctxt AnalysisCtxt) {
		injectUBool(&ctxt)
		g := ctxt.InitConf.Main()
		succs := ctxt.InitConf.GetSilentSuccessors(ctxt, g, ctxt.InitState)

		found := false
		for _, succ := range succs {
			node := succ.Configuration().GetUnsafe(g)
			if node.Panicked() {
				found = true
				break
			}
		}

		if !found {
			t.Error("Did not find a successor where", g, "has panicked")
		}
	}

	// Checks that the result of silent transitions only contains configurations where g is panicked
	checkMustPanic := func(t *testing.T, ctxt AnalysisCtxt) {
		injectUBool(&ctxt)
		g := ctxt.InitConf.Main()
		succs := ctxt.InitConf.GetSilentSuccessors(ctxt, g, ctxt.InitState)
		if len(succs) == 0 {
			t.Fatalf("0 successors? %v", succs)
		}

		for _, succ := range succs {
			node := succ.Configuration().GetUnsafe(g)
			if !node.Panicked() {
				t.Error("Found a successor where", g, "has not panicked")
				break
			}
		}
	}

	// Tests that the return value is greater than or equal to the provided value
	rvalGeq := func(v L.AbstractValue) absIntTestFunc {
		return testFactory(func(t *testing.T, _ defs.Goro, mem L.Memory) (L.Memory, func(L.AbstractValue)) {
			return mem, func(rval L.AbstractValue) {
				if !rval.Geq(v) {
					t.Errorf("Expected %v ⊒ %v", rval, v)
				}
			}
		})
	}

	rvalEq := func(v L.AbstractValue) absIntTestFunc {
		return testFactory(func(t *testing.T, _ defs.Goro, mem L.Memory) (L.Memory, func(L.AbstractValue)) {
			return mem, func(rval L.AbstractValue) {
				if !rval.Eq(v) {
					t.Errorf("Expected %v == %v", rval, v)
				}
			}
		})
	}

	tests := []absIntTest{
		{
			"param",
			`var ch chan int
			func tmain() chan int {
				return ch
			}`,
			paramEqualsReturnValue,
		},
		{
			"param_with_new",
			`var ch chan int
			func tmain() chan int {
				l := new(chan int)
				*l = ch
				return *l
			}`,
			paramEqualsReturnValue,
		},
		{
			"param_into_local",
			`var ch chan int
			func tmain() chan int {
				ch2 := ch
				return ch2
			}`,
			paramEqualsReturnValue,
		},
		{
			"param_into_local_then_branch",
			`var ch chan int
			func tmain() chan int {
				x := ch
				if true {
					return x
				}

		 		return ch
		 	}`,
			paramEqualsReturnValue,
		},
		{
			"funcall",
			`func id(ch chan int) chan int {
				return ch
			}

			var ch chan int
		 	func tmain() chan int {
		 		return id(ch)
		 	}`,
			paramEqualsReturnValue,
		},
		{
			"alloc_sync_channel",
			`func tmain() chan int {
				ch := make(chan int)
				return ch
			}`,
			simpleChannelTest(true, false),
		},
		{
			"alloc_sync_channel_loop",
			`func tmain() chan int {
				var ch chan int
				for i := 0; i < 2; i++ {
					ch = make(chan int)
				}
				return ch
			}`,
			simpleChannelTest(false, true),
		},
		{
			"alloc_sync_channel_goto",
			`var ubool bool
			func tmain() chan int {
				lbl:
				ch := make(chan int)
				if ubool {
					goto lbl
				}
				return ch
			}`,
			simpleChannelTest(false, false),
		},
		{
			"multialloc_bleed",
			`func tmain() int {
				var x *int
				for i := 0; i < 10; i++ {
					x = new(int)
				}
				return *x
			}`,
			rvalEq(L.Elements().AbstractBasic(int64(0))),
		},
		{
			"multialloc_bleed_2",
			`func tmain() bool {
				var x *int
				for i := 0; i < 10; i++ {
					x = new(int)
				}
				a := *x
				y := &a
				return y == y
			}`,
			rvalEq(L.Elements().AbstractBasic(true)),
		},
		{
			"param_into_global",
			`var g chan int
			var ch chan int

		 	func tmain() chan int {
		 		g = ch
		 		return g
		 	}`,
			paramEqualsReturnValue,
		},
		{
			"funcall_multiple_return",
			`func id(ch chan int) (chan int, error) {
				return ch, nil
			}

			var ch chan int
			func tmain() chan int {
				res, _ := id(ch)
				return res
			}`,
			paramEqualsReturnValue,
		},
		{
			"funcall_imprecise",
			`func f() bool { return true }
			func g() bool { return false }

			var ubool bool
			func tmain() bool {
				var h = f
				if ubool { h = g }
				return h()
			}`,
			arrTopTest,
		},
		{
			"simplest_closure",
			`func tmain() {
				x := 0
				clos := func(){
					println(x)
				}
				clos()
			}`,
			checkNoPanic,
		},
		{
			"closure_chan_passaround",
			`var ch chan int
			func tmain() chan int {
				ch := ch
				var loc chan int

		 		func() {
		 			loc = ch
		 		}()

		 		return loc
		 	}`,
			paramEqualsReturnValue,
		},
		{
			"closure_chan_passaround_2_layers",
			`var ch chan int
			func tmain() chan int {
				ch := ch
				var loc chan int

				func() {
					func() {
						loc = ch
					}()
				}()

				return loc
			}`,
			paramEqualsReturnValue,
		},
		{
			"go_freevar_transfer",
			`func tmain() {
				i := 10
				go func() {
					println(i)
				}()
			}`,
			checkNoPanic,
		},
		{
			"go_freevar_modify",
			`func tmain() {
				i := 10
				go func() {
					i = 20
				}()
				println(i)
			}`,
			checkNoPanic,
		},
		{
			"go_multiple",
			`func f() {}
			func g() {}
			var ubool bool
			func tmain() {
				var fun func()
				if ubool {
					fun = f
				} else {
					fun = g
				}
				go fun()
			}`,
			progressTest(false),
		},
		{
			"control_sensitivity",
			`var flag bool

			var ch chan int
			func tmain() chan int {
				if flag {
					ch = make(chan int)
				}

				return ch
			}`,
			paramEqualsReturnValue,
		},
		{
			"make_interface",
			`var ch chan int
			func tmain() chan int {
				var x interface{} = ch
				return x.(chan int)
			}`,
			paramEqualsReturnValue,
		},
		{
			"make_interface_with_commaok",
			`var ch chan int
			func tmain() chan int {
				var x interface{} = ch
				if ch, ok := x.(chan int); ok {
					return ch
				} else {
					return make(chan int)
				}
			}
			`,
			paramEqualsReturnValue,
		},
		{
			"struct_addr_of_field",
			`type S struct {
				ch chan int
			}

			var ch chan int
			func tmain() chan int {
				s := S{}
				p := &s.ch
				*p = ch
				return s.ch
			}`,
			paramEqualsReturnValue,
		},
		{
			"struct_nested_direct",
			`type inner struct {
				ch chan int
			}

			type outer struct { inner }

			var ch chan int
			func tmain() chan int {
				o := outer{}
				o.ch = ch
				return o.ch
			}`,
			paramEqualsReturnValue,
		},
		{
			"struct_nested_indirect",
			`type inner struct {
				ch chan int
			}

			type outer struct { *inner }

			var ch chan int
			func tmain() chan int {
				o := outer{new(inner)}
				o.ch = ch
				return o.ch
			}`,
			paramEqualsReturnValue,
		},
		{
			"defer_builtin",
			`func tmain() {
				defer println()
			}`,
			checkNoPanic,
		},
		{
			"defer_userfunc",

			`func f(i int) {
				println(i)
			}

			func tmain() {
				defer f(10)
			}`,
			checkNoPanic,
		},
		{
			"defer_pass_value",
			// Since we're passing ch as an argument to the deferred function,
			// it should not be affected by the later assignment to ch.
			`var gch chan int

			func f(ch chan int) {
				defer func(x chan int) {
					gch = x
				}(ch)

				ch = make(chan int)
			}

			var ch chan int
			func tmain() chan int {
				f(ch)
				return gch
			}`,
			paramEqualsReturnValue,
		},
		{
			"defer_pass_reference",
			// Here fch is captured by the deferred function. Therefore the
			// update of fch should be reflected when the deferred function is called.
			`var gch chan int

			func f(ch chan int) {
				fch := make(chan int)

				defer func() {
					gch = fch
				}()

				fch = ch
			}

			var ch chan int
			func tmain() chan int {
				f(ch)
				return gch
			}`,
			paramEqualsReturnValue,
		},
		{
			"array_assign",
			`func tmain() [4]int {
				arr := [4]int{}
				arr[0] = 2
				println(arr[1])
				return arr
			}`,
			arrTopTest,
		},
		{
			"array_ptr",
			`func tmain() {
				arr := [4]int{}
				println(&arr[2])
			}`,
			checkNoPanic,
		},
		{
			"slice_basic",
			`func tmain() {
				sl := make([]int, 4)
				sl[2] = 3
				println(&sl[1])
			}`,
			checkNoPanic,
		},
		{
			"slice_from_array",
			`func tmain() [4]int {
				arr := [4]int{}
				sl := arr[2:3]
				sl[0] = 10
				return arr
			}`,
			arrTopTest,
		},
		{
			"slice_dynamic_len",
			`var ubool bool
			func tmain() {
				le := 5
				if ubool { le = 3 }
				sl := make([]int, le)
				println(sl[2])
			}`,
			checkNoPanic,
		},
		{
			"array_range",
			`func tmain() (int, int) {
				arr := [4]int{}
				var i, v int
				for i, v = range arr {
					println(i, v)
				}
				return i, v
			}`,
			testFactory(func(t *testing.T, _ defs.Goro, mem L.Memory) (L.Memory, func(L.AbstractValue)) {
				return mem, func(rval L.AbstractValue) {
					if !rval.IsStruct() {
						t.Fatal("Expected", rval, "to be a struct")
					}

					sval := rval.StructValue()
					if idx := sval.Get(0).AbstractValue().BasicValue(); !idx.IsTop() {
						t.Error("Expected", idx, "to be ⊤")
					}

					if elem := sval.Get(1).AbstractValue().BasicValue(); !elem.Is(int64(0)) {
						t.Error("Expected", elem, "to be 0")
					}
				}
			}),
		},
		{
			"map_basics",
			`func tmain() (bool, bool) {
				mp := map[int]bool{}
				mp[2] = true
				println(mp[4])
				v, ok := mp[1]
				return v, ok
			}`,
			testFactory(func(t *testing.T, _ defs.Goro, mem L.Memory) (L.Memory, func(L.AbstractValue)) {
				return mem, func(rval L.AbstractValue) {
					if !rval.IsStruct() {
						t.Fatal("Expected", rval, "to be a struct")
					}

					sval := rval.StructValue()
					if val := sval.Get(0).AbstractValue().BasicValue(); !val.IsTop() {
						t.Error("Expected", val, "to be ⊤")
					}

					if ok := sval.Get(1).AbstractValue().BasicValue(); !ok.IsTop() {
						t.Error("Expected", ok, "to be ⊤")
					}
				}
			}),
		},
		{
			"map_iterator",
			`func tmain() *int {
				mp := map[*int]int{
					new(int): 3,
					new(int): 6,
				}

				var key *int
				var value int
				for key, value = range mp {
					println(value)
				}

				return key
			}`,
			testFactory(func(t *testing.T, _ defs.Goro, mem L.Memory) (L.Memory, func(L.AbstractValue)) {
				return mem, func(rval L.AbstractValue) {
					if !rval.IsPointer() {
						t.Fatal("Expected", rval, "to be a points-to set")
					}

					ptsto := rval.PointerValue()
					if !ptsto.Contains(loc.NilLocation{}) {
						t.Error("Expected", ptsto, "to contain nil")
					}

					ptsto = ptsto.Remove(loc.NilLocation{})
					if ptsto.Size() != 2 {
						t.Error("Expected", rval, "to include 2 pointers")
					}
				}
			}),
		},
		{
			"map_iterator_empty",
			`func tmain() {
				mp := map[int]int{}
				var key, value int
				for key, value = range mp {
				}
				println(key, value)
			}`,
			checkNoPanic,
		},
		{
			"looptop",
			`func tmain() bool {
				var b bool
				for _ = range []int{} {
					b = true
				}
				return b
			}`,
			arrTopTest,
		},
		{
			"phi",
			`var ubool bool
			func tmain() bool {
				// Short-circuit introduces phi
				return ubool || !ubool
			}`,
			arrTopTest,
		},
		{
			"phi_or_precision",
			`var x int
			func tmain() bool {
				x = 2
				return x < 2 || x > 2
			}`,
			rvalEq(Elements().AbstractBasic(false)),
		},
		{
			"phi_and_precision",
			`var x int
			func tmain() bool {
				x = 2
				return x <= 2 && x >= 2
			}`,
			rvalEq(Elements().AbstractBasic(true)),
		},
		{
			"interface_mix",
			`var ubool bool
			func tmain() interface{} {
				var i interface{}

				if ubool {
					i = 10
				} else {
					i = make(chan int)
				}

				return i
			}`,
			testFactory(func(t *testing.T, _ defs.Goro, mem L.Memory) (L.Memory, func(L.AbstractValue)) {
				return mem, func(rval L.AbstractValue) {
					if !rval.IsPointer() {
						t.Fatal("Expected", rval, "to be pointer")
					}

					if rval.PointerValue().Size() != 2 {
						t.Error(rval, "should contain two pointers")
					}
				}
			}),
		},
		{
			"closure_fun_mix",
			`func f() {}
			var ubool bool
			func tmain() {
				ubool := ubool
				var fun func()
				if ubool {
					fun = f
				} else {
					fun = func() {
						// Closure with capture
						println(ubool)
					}
				}

				fun()
			}`,
			checkNoPanic,
		},
		{
			"closure_freevar_type_mixup",
			`var ubool bool
			func tmain() {
				ubool := ubool
				i := new(int)

				var fun func()
				if ubool {
					fun = func() { println(i) }
				} else {
					fun = func() { println(ubool) }
				}

				fun()
			}`,
			checkNoPanic,
		},
		{
			"closure_escaped",
			`func external(func())
			func tmain() int {
				i := 0
				f := func() {
					i++
				}

				external(f)
				f()
				return i
			}`,
			rvalEq(L.Consts().BasicTopValue()),
		},
		{
			"itf_escaped",
			`func external(any)
			func tmain() int {
				var i any = 10
				external(i)
				return i.(int)
			}`,
			rvalEq(L.Elements().AbstractBasic(int64(10))),
		},
		{
			"go_builtin",
			`func tmain() {
				i := 20
				go println(i)
			}`,
			checkNoPanic,
		},
		{
			"multireturn",
			`func f() {}

			func tmain() {
				f()
				// Generate some ssa instructions
				x := 1
				y := 2
				z := 3
				f()
				println(x, y, z)
			}`,
			checkNoPanic,
		},
		{
			"recursion",
			`func f(n int) int {
				if n <= 1 {
					return 1
				}

				return n * f(n-1)
			}

			func tmain() int {
				return f(5)
			}`,
			rvalGeq(Elements().AbstractBasic(int64(5 * 4 * 3 * 2))),
		},
		{
			"defer_twice_soundness",
			`var x int
			func f(i int) {
				x = i
			}

			func tmain() {
				defer f(2)
				defer f(3)
			}`,
			func(t *testing.T, ctxt AnalysisCtxt) {
				tid := ctxt.InitConf.Main()
				succs := ctxt.InitConf.GetTransitions(ctxt, ctxt.InitState)
				if len(succs) != 1 {
					t.Fatal("Not exactly one possible successor?", succs)
				}

				glob := loc.GlobalLocation{Site: tid.CtrLoc().Node().Function().Pkg.Var("x")}

				for _, succ := range succs {
					mem := succ.State.Memory()
					xval, found := mem.Get(glob)
					if !found {
						t.Fatal("Missing value for", glob)
					}

					two := Elements().AbstractBasic(int64(2))
					if !two.Leq(xval) {
						t.Error("Expected", two, "⊑", xval)
					}
				}
			},
		},
		{
			"defer_under_cond",
			`func f(i int) {
				println(i)
			}

			func tmain() {
				if false {
					i := 10
					defer f(i)
				}
			}`,
			checkNoPanic,
		},
		{
			"defer_under_cond2",
			`func f(i int) {
				println(i)
			}

			func tmain() {
				if false {
					i := 10
					defer f(i)
				} else {
					defer f(5)
				}

				defer f(10)
			}`,
			checkNoPanic,
		},
		{
			"defer_under_cond3",
			`var ubool bool
			func tmain() {
				if ubool {
					i := 10
					defer func() { println(i) }()
				} else {
					defer println(5)
				}

				defer println(10)
			}`,
			checkNoPanic,
		},
		{
			"defer_under_cond4",
			`func external()
			var ubool bool
			func tmain() {
				if ubool {
					defer println(10)
				} else {
					defer println(5)
				}

				defer external()
			}`,
			checkNoPanic,
		},
		{
			"fun_mix",
			`func f() {}
			func g() {}
			var ubool bool
			func tmain() {
				var fun func()
				if ubool {
					fun = f
				} else {
					fun = g
				}

				fun()
			}`,
			checkNoPanic,
		},
		{
			"fun_mix_global",
			`var fun func()
			var ubool bool
			func tmain() {
				if ubool {
					fun = func() { }
				}
			}`,
			checkNoPanic,
		},
		{
			"unsafe_sliceheader",
			`import (
				"unsafe"
			)

			type SliceHeader struct {
				Data uintptr
				Len  int
				Cap  int
			}

			func tmain() int {
				x := []int{3}
				return (*SliceHeader)(unsafe.Pointer(&x)).Len
			}`,
			arrTopTest,
		},
		{
			"unsafe_conversion_causes_top_allocation",
			`import "unsafe"
			func tmain() int {
				x := new(int)
				y := (*int)(unsafe.Pointer(x))
				return *y
			}`,
			arrTopTest,
		},
		{
			"interface_call",
			`type I interface {
				GetInt() int
			}

			type impl struct {x int}
			func (i impl) GetInt() int {
				return i.x
			}

			func tmain() {
				vimpl := impl{x: 20}
				var vI I = vimpl
				println(vimpl.GetInt())
				println(vI.GetInt())
			}`,
			checkNoPanic,
		},
		{
			"interface_call_ptr_receiver",
			`type I interface {
				GetCh() chan int
			}

			type impl struct {ch chan int}
			func (i *impl) GetCh() chan int { return i.ch }

			var ch chan int
			func tmain() chan int {
				var vI I = &impl{ch: ch}
				return vI.GetCh()
			}`,
			paramEqualsReturnValue,
		},
		{
			"call_generic_function",
			`func f[E any](x E) E {
				return x
			}

			func tmain() int {
				f("hi")
				return f(10)
			}`,
			rvalEq(L.Elements().AbstractBasic(int64(10))),
		},
		{
			"slice_append",
			`func tmain() chan int {
				ch := make(chan int)
				chans := []chan int{}
				chans = append(chans, ch)
				ch = chans[0]
				return ch
			}`,
			simpleChannelTest(true, true),
		},
		{
			"empty_slice_append",
			`func tmain() {
				var is []int
				_ = append(is, 10)
			}`,
			checkNoPanic,
		},
		{
			"slice_named_append",
			`type intList []int
			var ubool bool
			func tmain() int {
				le := 4
				if ubool { le = 5 }
				var l intList = make(intList, le)
				l = append(l, 123)
				return l[4]
			}`,
			arrTopTest,
		},
		{
			"slice_named_append2",
			`type intList []int
			func tmain() int {
				var l intList
				l = append(l, 123)
				return l[0]
			}`,
			checkMayPanic,
		},
		{
			"deref_nil",
			`func tmain() {
				var x *int
				println(*x)
			}`,
			checkMayPanic,
		},
		{
			"store_on_nil",
			`func tmain() {
				var x *int
				*x = 10
			}`,
			checkMayPanic,
		},
		{
			"store_on_maybe_nil",
			`var ubool bool
			func tmain() {
				var x *int
				if ubool { x = new(int) }
				*x = 10
			}`,
			checkMayPanic,
		},
		{
			"address_of_field_from_nil",
			`type T struct { x int }
			func tmain() {
				var t *T
				println(&t.x)
			}`,
			checkMayPanic,
		},
		{
			"address_of_element_from_maybe_nil_slice",
			`var ubool bool
			func tmain() {
				var x []int
				if ubool { x = []int{1, 2, 3} }
				println(&x[1])
			}`,
			checkMayPanic,
		},
		{
			"address_of_element_from_nil_slice",
			`func tmain() *int {
				var a []int
				return &a[0]
			}`,
			testFactory(func(t *testing.T, _ defs.Goro, mem L.Memory) (L.Memory, func(L.AbstractValue)) {
				return mem, func(rval L.AbstractValue) {
					typ := rval.PointsTo().Entries()[0].Type()
					if typ.String() != "*int" {
						t.Errorf("Expected %v to be *int", typ)
					}
				}
			}),
		},
		{
			"slice_cap_top",
			`func tmain() int {
				s := []int{}
				return cap(s)
			}`,
			rvalEq(L.Consts().BasicTopValue()),
		},
		{
			"chan_cap_nil",
			`func tmain() int {
				var ch chan int
				return cap(ch)
			}`,
			rvalEq(L.Elements().AbstractBasic(int64(0))),
		},
		{
			"chan_cap_0",
			`func tmain() int {
				ch := make(chan int, 0)
				return cap(ch)
			}`,
			rvalEq(L.Elements().AbstractBasic(int64(0))),
		},
		{
			"chan_cap_1",
			`func tmain() int {
				ch := make(chan int, 1)
				return cap(ch)
			}`,
			rvalEq(L.Elements().AbstractBasic(int64(1))),
		},
		{
			"chan_cap_certain",
			`var ubool bool

			func tmain() int {
				var ch chan int
				if ubool {
					ch = make(chan int, 1)
				} else {
					ch = make(chan int, 1)
				}
				return cap(ch)
			}`,
			rvalEq(L.Elements().AbstractBasic(int64(1))),
		},
		{
			"chan_cap_uncertain",
			`var ubool bool

			func tmain() int {
				var ch chan int
				if ubool {
					ch = make(chan int, 1)
				}
				return cap(ch)
			}`,
			rvalEq(L.Consts().BasicTopValue()),
		},
		{
			"chan_cap_uncertain_2",
			`var ubool bool

			func tmain() int {
				var ch chan int
				if ubool {
					ch = make(chan int, 1)
				} else {
					ch = make(chan int, 2)
				}
				return cap(ch)
			}`,
			rvalEq(L.Consts().BasicTopValue()),
		},
		{
			"builtin_panic",
			`func tmain() {
				panic("oh no")
			}`,
			checkMayPanic,
		},
		{
			"map_write_nil",
			`func tmain() {
				var m map[int]int
				m[2] = 10
			}`,
			checkMayPanic,
		},
		{
			"map_get_nil",
			`func tmain() {
				var m map[int]int
				println(m[10])
			}`,
			checkNoPanic,
		},
		{
			"map_range_maybe_nil",
			`var ubool bool
			func tmain() {
				var m map[int]int
				if ubool { m = map[int]int{1: 10, 2: 24} }
				for k, v := range m {
					println(k, v)
				}
			}`,
			checkNoPanic,
		},
		{
			"slice_range_nil",
			`func tmain() {
				var s []int
				for i, v := range s {
					println(i, v)
				}
			}`,
			// Range over slice is translated into a for-loop. We will try to access
			// an index of a nil slice, which panics.
			checkMayPanic,
		},
		{
			"interface_typeassert_maybe_nil",
			`var ubool bool
			func tmain() {
				var x interface{}
				if ubool { x = 2 }
				println(x.(int))
			}`,
			checkMayPanic,
		},
		{
			"interface_typeassert_wrong_type_may_panic",
			`var ubool bool
			func tmain() {
				var x interface{}
				if ubool {
					x = 10
				} else {
					x = "abc"
				}
				println(x.(int))
			}`,
			checkMayPanic,
		},
		{
			"interface_typeassert_commaok_no_panic",
			`var ubool bool
			func tmain() {
				var x interface{}
				if ubool {
					x = 10
				} else {
					x = "abc"
				}

				if v, ok := x.(int); ok {
					println(v)
				}
			}`,
			checkNoPanic,
		},
		{
			"interface_typeassert_nil_commaok_no_panic",
			`func tmain() {
				var x interface{}
				_, ok := x.(int)
				println(ok)
			}`,
			checkNoPanic,
		},
		{
			"interface_typeassert_to_itf_commaok_succeeds",
			`type T interface { get() int }
			type impl int
			func (x impl) get() int { return int(x) }

			func make() any { return impl(10) }

			func tmain() (int, bool) {
				itf := make()
				i, ok := itf.(T)
				return i.get(), ok
			}`,
			rvalEq(Elements().AbstractStructV(
				Elements().AbstractBasic(int64(10)),
				Elements().AbstractBasic(true),
			)),
		},
		{
			"interface_typeassert_to_itf_commaok_must_fail",
			`type T interface { fget() int }
			type impl int
			func (x impl) get() int { return int(x) }

			func make() any { return impl(10) }

			func tmain() bool {
				itf := make()
				_, ok := itf.(T)
				return ok
			}`,
			rvalEq(Elements().AbstractBasic(false)),
		},
		{
			"interface_typeassert_to_itf_must_fail",
			`type T interface { fget() int }
			type impl int
			func (x impl) get() int { return int(x) }

			func make() any { return impl(10) }

			func tmain() {
				itf := make()
				i := itf.(T)
				println(i)
			}`,
			checkMustPanic,
		},
		{
			"[disabled] panicked_return",
			`func f() {
				panic("Oh no")
			}

			func tmain() {
				f()
			}`,
			checkMayPanic,
		},
		{
			"[disabled] panic_into_defer",
			`func f() { }

			var ubool bool
			func tmain() {
				if ubool {
					defer f()
				} else {
					defer f()
				}
				panic("Oh no")
			}`,
			checkMayPanic,
		},
		{
			"interface_call_on_nil",
			`type I interface { GetInt() int }

			func tmain() {
				var i I
				println(i.GetInt())
			}`,
			checkMayPanic,
		},
		{
			"interface_call_on_maybe_nil",
			`type I interface { GetInt() int }
			type impl struct { }
			func (impl) GetInt() int { return 2 }

			var ubool bool
			func tmain() {
				var i I
				if ubool { i = impl{} }
				println(i.GetInt())
			}`,
			checkMayPanic,
		},
		{
			"interface_call_same_target_two_receivers",
			`type I interface { GetInt() int }
			type impl struct { }
			func (impl) GetInt() int { return 10 }

			var ubool bool
			func tmain() {
				var i I = impl{}
				if ubool { i = impl{} }
				println(i.GetInt())
			}`,
			checkNoPanic,
		},
		{
			"invoke_to_embedded_through_anonymous_struct",
			`type I interface { f() }
			type x int
			func (x) f() { }

			func tmain() {
				var i I = struct{ x }{ 10 }
				i.f()
				type s = struct{ x }
				i = s{ 9 }
				i.f()
			}`,
			checkNoPanic,
		},
		{
			"invoke_ambiguous_on_named_type",
			`type I interface { f() }
			type a int
			func (a) f() { panic("Oh no") }
			type b a
			func (b) f() { }

			func tmain() {
				var i I = b(10)
				i.f()
			}`,
			checkNoPanic,
		},
		{
			"invoke_on_externally_linked",
			`type I interface { f() }
			type a int
			func (a) f()

			func tmain() {
				var i I = a(10)
				i.f()
			}`,
			checkNoPanic,
		},
		{
			"changeinterface",
			`type base interface { GetInt() int }
			type super interface { base ; GetString() string }
			func tmain() {
				var s super
				var b base = s
				_ = b
			}`,
			checkNoPanic,
		},
		{
			"lookup_on_string",
			`func tmain() {
				str := "Hello, World!"
				println(str[4])
			}`,
			checkNoPanic,
		},
		{
			"string_iterator",
			`func tmain() rune {
				var i int
				var r rune
				for i, r = range "Hello, World!" {
					println(i)
				}
				return r
			}`,
			arrTopTest,
		},
		{
			"string_iterator_empty",
			`func tmain() {
				for i, r := range "" {
					println(i, r)
				}
			}`,
			checkNoPanic,
		},
		{
			"string_to_[]byte_lookup",
			`func tmain() {
				s := "abc"
				b := []byte(s)
				p := &b[0]
				println(*p)
			}`,
			checkNoPanic,
		},
		{
			"nil_comp_precision",
			`func tmain() {
				var x *int = new(int)
				if x == nil { panic("oh no") }
				if nil == x { panic("oh no") }
				var y *int
				if y != nil { panic("oh no") }
				if nil != y { panic("oh no") }
			}`,
			checkNoPanic,
		},
		{
			"ptrcomp_soundness_1",
			`var ubool bool
			func tmain() bool {
				var t *int
				if ubool {
					t = new(int)
				} else {
					t = new(int)
				}
				return t == t
			}`,
			arrTopTest,
		},
		{
			"ptrcomp_soundness_2",
			`var ubool bool
			func tmain() bool {
				var t *int
				if ubool { t = new(int) }
				return t == nil
			}`,
			arrTopTest,
		},
		{
			"ptrcomp_soundness_3",
			`func f() *int {
				return new(int)
			}

			func tmain() bool {
				return f() == f()
			}`,
			arrTopTest,
		},
		{
			"ptrcomp_soundness_4",
			`func f() *int {
				return new(int)
			}

			func tmain() bool {
				return f() != f()
			}`,
			arrTopTest,
		},
		{
			"integer_lt_precision",
			`func tmain() {
				var i int
				if i < 0 { panic("oh no") }
			}`,
			checkNoPanic,
		},
		{
			"itf_nil_comp_precision",
			`func tmain() {
				var x, y interface{}
				if x != y { panic("Oh no") }
				if !(x == y) { panic("Oh no") }
				if x != nil { panic("Oh no") }
				if !(y == nil) { panic("Oh no") }
			}`,
			checkNoPanic,
		},
		{
			"itfcomp_precision_1",
			`func tmain() {
				var i *int
				var x, y interface{}
				y = i
				if x == y { panic("Oh no") }
				if !(x != y) { panic("Oh no") }
				x = 0
				y = 1
				if x == y { panic("Oh no") }
			}`,
			checkNoPanic,
		},
		{
			"itfcomp_precision_2",
			`func tmain() {
				if interface{}(0) != interface{}(0) { panic("Oh no") }
			}`,
			checkNoPanic,
		},
		{
			"itfcomp_precision_3",
			`func tmain() {
				var a *float64
				var b *int64
				x := interface{}(a)
				y := interface{}(b)
				if x == y { panic("Oh no") }
			}`,
			checkNoPanic,
		},
		{
			"itfcomp_soundness_1",
			`var ubool bool
			func tmain() bool {
				var x, y interface{}
				if ubool { y = true }
				return x == y
			}`,
			arrTopTest,
		},
		{
			"itfcomp_soundness_2",
			`var ubool bool
			func tmain() bool {
				x := interface{}(ubool)
				y := x
				return x == y
			}`,
			rvalGeq(Elements().AbstractBasic(true)),
		},
		{
			"itfcomp_soundness_3",
			`func f(i int) interface{} { return i }

			func tmain() bool {
				return f(0) == f(1)
			}`,
			rvalGeq(Elements().AbstractBasic(false)),
		},
		{
			"itfcomp_soundness_4",
			`func f(i int) interface{} { return i }

			func tmain() bool {
				f(1)
				x := f(0)
				return x != x
			}`,
			rvalGeq(Elements().AbstractBasic(false)),
		},
		{
			"itfcomp_soundness_5",
			`var ubool bool
			func tmain() bool {
				var x interface{} = 0
				if ubool { x = 1 }
				return x == interface{}(0)
			}`,
			arrTopTest,
		},
		{
			"itfcomp_soundness_6",
			`import "errors"
			var ubool bool
			func f() error {
				if ubool { return errors.New("asd") }
				return nil
			}
			func tmain() bool {
				return f() == nil
			}`,
			arrTopTest,
		},
		{
			"itfcomp_soundness_7",
			`import "errors"
			type e struct { }
			func (e) Error() string { return "asd" }

			var ubool bool
			func tmain() bool {
				var x error = e{}
				err := errors.New("ASD")
				if ubool { err = x }
				return err == x
			}`,
			arrTopTest,
		},
		{
			"ssa:wrapnilchk",
			// We want to test that we have precise handling of ssa:wrapnilchk
			`type I interface {
				getCh() chan int
			}

			type C struct {
				ch chan int
			}
			// Plain struct receiver
			func (c C) getCh() chan int { return c.ch }

			var ch chan int
			func tmain() chan int {
				// Use pointer in itf to target wrapper function
				var i I = &C{ch}
				return i.getCh()
			}`,
			paramEqualsReturnValue,
		},
		{
			"RLocker-minimal",
			// Tests functionality similar to what RWMutex.RLocker needs
			`type I interface { f() }

			type A struct {}
			func (*A) f() {
				panic("Oh no")
			}
			func (*A) g() {
				println("this is good")
			}
			func (a *A) toB() I {
				return (*B)(a)
			}

			type B A
			func (b *B) f() {
				(*A)(b).g()
			}

			func tmain() {
				a := &A{}
				i := a.toB()
				i.f()
			}`,
			checkNoPanic,
		},
		{
			"functionptr-passed-to-external",
			`func external(func())
			func tmain() int {
				i := 10
				f := func() { println(10) }
				g := func() { i = 20 }
				external(f)
				external(g)
				return i
			}`,
			arrTopTest,
		},
		{
			// Checks that deferring calls and calling functions inside
			// deferred calls work normally after runtime.Goexit is encountered
			"runtime-goexit",
			`import "runtime"
			var (
				normalDefer, deferCall, normalDeferAfterCall,
				deferCallDefer, deferCallDeferCall,
				deferCallDeferAfter, exitAfterGoExit int
			)
			func f() {
				defer g()
				deferCall = 1
			}
			func g() {
				deferCallDefer = 1
				func() {
					deferCallDeferCall = 1
				}()
				deferCallDeferAfter = 1
			}
			func tmain() {
				defer func() {
					normalDefer = 1
					f()
					normalDeferAfterCall = 1
				}()

				exitAfterGoExit = 1
				runtime.Goexit()
				exitAfterGoExit = 2
			}`,
			testFactoryWMem(func(t *testing.T, g defs.Goro, mem L.Memory) (
				L.Memory, func(L.Memory, L.AbstractValue)) {
				return mem, func(mem L.Memory, _ L.AbstractValue) {
					pkg := g.CtrLoc().Root().Pkg
					for name, member := range pkg.Members {
						if global, ok := member.(*ssa.Global); ok && !strings.ContainsRune(name, '$') {
							val := mem.GetUnsafe(loc.GlobalLocation{Site: global})
							if !val.IsBasic() {
								t.Errorf("Expected %s:%v to be basic", name, val)
							} else if bval := val.BasicValue(); bval.IsBot() || bval.IsTop() {
								t.Errorf("Expected %s:%v to be definite", name, bval)
							} else if !bval.Is(int64(1)) {
								t.Errorf("Expected %s:%v to be 1", name, bval)
							}
						}
					}
				}
			}),
		},
		{
			"unop_int_sub",
			`var x int
			func tmain() int {
				x = 1
				return -x
			}`,
			rvalEq(Elements().AbstractBasic(int64(-1))),
		},
		{
			"intdiv_by_zero",
			`var x int
			func tmain() int { return 2 / x }`,
			checkMustPanic,
		},
		{
			"intmod_by_zero",
			`var x int
			func tmain() int { return 2 % x }`,
			checkMustPanic,
		},
		{
			"[disabled] andersen-unsoundness-crash",
			// The CFG does not contain a call edge for `j.f()`
			// because the andersen analysis is unsound
			`import "sync/atomic"

			type I interface { f() }
			type A struct {}
			func (A) f() { }

			func tmain() {
				var v atomic.Value
				var i I = A{}
				v.Store(i)
				j := v.Load().(I)
				j.f()
			}`,
			checkNoPanic,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if strings.HasPrefix(test.name, "[disabled]") {
				t.SkipNow()
			}

			defer func() {
				if err := recover(); err != nil {
					t.Errorf("Panic while analyzing...\n%v\n%s\n", err, debug.Stack())
				}
			}()

			test.doTest(t, loadProgram(t, `package main

			`+test.content+`

			func main() {
				tmain()
			}`))
		})
	}
}
