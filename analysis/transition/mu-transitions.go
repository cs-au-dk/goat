package transition

import (
	"fmt"

	"github.com/cs-au-dk/goat/analysis/defs"
	loc "github.com/cs-au-dk/goat/analysis/location"
	"github.com/cs-au-dk/goat/utils"
)

type Lock struct {
	transitionSingle
	Mu loc.Location
}

func (t Lock) PrettyPrint() {
	fmt.Println("Locking", t.Mu, "on thread", t.progressed)
}

func (t Lock) String() (str string) {
	return t.progressed.String() + "-[ Lock(" + t.Mu.String() + ") ]"
}

func (t Lock) Hash() uint32 {
	return utils.HashCombine(t.progressed.Hash(), t.Mu.Hash())
}

func NewLock(progressed defs.Goro, mu loc.Location) Lock {
	return Lock{transitionSingle{progressed}, mu}
}

type Unlock struct {
	transitionSingle
	Mu loc.Location
}

func (t Unlock) PrettyPrint() {
	fmt.Println("Unlocking", t.Mu, "on thread", t.progressed)
}

func (t Unlock) String() (str string) {
	return t.progressed.String() + "-[ Unlock(" + t.Mu.String() + ") ]"
}

func (t Unlock) Hash() uint32 {
	return utils.HashCombine(t.progressed.Hash(), t.Mu.Hash())
}

func NewUnlock(progressed defs.Goro, mu loc.Location) Unlock {
	return Unlock{transitionSingle{progressed}, mu}
}

type RLock struct {
	transitionSingle
	Mu loc.Location
}

func (t RLock) PrettyPrint() {
	fmt.Println("Read locking", t.Mu, "on thread", t.progressed)
}

func (t RLock) String() (str string) {
	return t.progressed.String() + "-[ RLock(" + t.Mu.String() + ") ]"
}

func (t RLock) Hash() uint32 {
	return utils.HashCombine(t.progressed.Hash(), t.Mu.Hash())
}

func NewRLock(progressed defs.Goro, mu loc.Location) RLock {
	return RLock{transitionSingle{progressed}, mu}
}

type RUnlock struct {
	transitionSingle
	Mu loc.Location
}

func (t RUnlock) PrettyPrint() {
	fmt.Println("Read unlocking", t.Mu, "on thread", t.progressed)
}

func (t RUnlock) String() (str string) {
	return t.progressed.String() + "-[ RUnlock(" + t.Mu.String() + ") ]"
}

func (t RUnlock) Hash() uint32 {
	return utils.HashCombine(t.progressed.Hash(), t.Mu.Hash())
}

func NewRUnlock(progressed defs.Goro, mu loc.Location) RUnlock {
	return RUnlock{transitionSingle{progressed}, mu}
}
