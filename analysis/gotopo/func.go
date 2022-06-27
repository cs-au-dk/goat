package gotopo

import (
	"strings"

	"golang.org/x/tools/go/ssa"
)

// Every function may actively use some channels or
// synchronization primitives. The channels and synchronization
// primitives are separated
type Func struct {
	inflowChans  map[ssa.Value]struct{}
	createdChans map[ssa.Value]struct{}
	outflowChans map[ssa.Value]struct{}
	usedChans    map[ssa.Value]struct{}
	usedSync     map[ssa.Value]struct{}
}

func (f *Func) init() {
	f.usedChans = make(map[ssa.Value]struct{})
	f.inflowChans = make(map[ssa.Value]struct{})
	f.outflowChans = make(map[ssa.Value]struct{})
	f.createdChans = make(map[ssa.Value]struct{})
	f.usedSync = make(map[ssa.Value]struct{})
}

func (f *Func) String() (str string) {
	if len(f.usedChans) > 0 {
		str += colorize.ChanSet("Used channels") + ": {"
		strs := []string{}
		for ch := range f.usedChans {
			strs = append(strs, colorize.Chan(ch.Name()+" = "+ch.String()))
		}

		str += strings.Join(strs, ", ")
		str += "}\n"
	}
	if len(f.createdChans) > 0 {
		str += colorize.ChanSet("Created channels") + ": {"
		strs := []string{}
		for ch := range f.createdChans {
			strs = append(strs, colorize.Chan(ch.Name()+" = "+ch.String()))
		}

		str += strings.Join(strs, ", ")
		str += "}\n"
	}
	if len(f.inflowChans) > 0 {
		str += colorize.ChanSet("In-flowing channels") + ": {"
		strs := []string{}
		for ch := range f.inflowChans {
			strs = append(strs, colorize.Chan(ch.Name()+" = "+ch.String()))
		}

		str += strings.Join(strs, ", ")
		str += "}\n"
	}
	if len(f.outflowChans) > 0 {
		str += colorize.ChanSet("Out-flowing channels") + ": {"
		strs := []string{}
		for ch := range f.outflowChans {
			strs = append(strs, colorize.Chan(ch.Name()+" = "+ch.String()))
		}

		str += strings.Join(strs, ", ")
		str += "}\n"
	}
	if len(f.usedSync) > 0 {
		str += colorize.SyncSet("Used sync") + ": {"

		strs := []string{}
		for s := range f.usedSync {
			strs = append(strs, colorize.Chan(s))
		}

		str += strings.Join(strs, ", ") + "}\n"
	}

	return
}

func newFunc() (f *Func) {
	f = new(Func)
	f.init()
	return
}

func (f *Func) Chans() map[ssa.Value]struct{} {
	return f.usedChans
}

func (f *Func) OutChans() map[ssa.Value]struct{} {
	return f.outflowChans
}

func (f *Func) AddUseChan(v ssa.Value) {
	f.usedChans[v] = struct{}{}
}

func (f *Func) AddInChan(v ssa.Value) {
	f.inflowChans[v] = struct{}{}
}

func (f *Func) AddOutChan(v ssa.Value) {
	f.outflowChans[v] = struct{}{}
}

func (f *Func) AddCreatedChan(v ssa.Value) {
	f.createdChans[v] = struct{}{}
}

func (f *Func) AddUseSync(v ssa.Value) {
	f.usedSync[v] = struct{}{}
}

func (f *Func) Sync() map[ssa.Value]struct{} {
	return f.usedSync
}

func (f *Func) IsActive(v ssa.Value) bool {
	_, ok := f.usedChans[v]
	if ok {
		return ok
	}
	_, ok = f.usedSync[v]
	return ok
}

func (f *Func) HasChan(v ssa.Value) bool {
	_, ok := f.usedChans[v]
	if ok {
		return ok
	}
	_, ok = f.outflowChans[v]
	return ok
}
