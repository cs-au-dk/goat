package transition

import (
	"fmt"

	"github.com/cs-au-dk/goat/analysis/defs"
	loc "github.com/cs-au-dk/goat/analysis/location"
	"github.com/cs-au-dk/goat/utils"
)

type Lock struct {
	Progressed defs.Goro
	Mu         loc.Location
}

func (t Lock) PrettyPrint() {
	fmt.Println("Locking", t.Mu, "on thread", t.Progressed)
}

func (t Lock) String() (str string) {
	return t.Progressed.String() + "-[ Lock(" + t.Mu.String() + ") ]"
}

func (t Lock) Hash() uint32 {
	return utils.HashCombine(t.Progressed.Hash(), t.Mu.Hash())
}

type Unlock struct {
	Progressed defs.Goro
	Mu         loc.Location
}

func (t Unlock) PrettyPrint() {
	fmt.Println("Unlocking", t.Mu, "on thread", t.Progressed)
}

func (t Unlock) String() (str string) {
	return t.Progressed.String() + "-[ Unlock(" + t.Mu.String() + ") ]"
}

func (t Unlock) Hash() uint32 {
	return utils.HashCombine(t.Progressed.Hash(), t.Mu.Hash())
}

type RLock struct {
	Progressed defs.Goro
	Mu         loc.Location
}

func (t RLock) PrettyPrint() {
	fmt.Println("Read locking", t.Mu, "on thread", t.Progressed)
}

func (t RLock) String() (str string) {
	return t.Progressed.String() + "-[ RLock(" + t.Mu.String() + ") ]"
}

func (t RLock) Hash() uint32 {
	return utils.HashCombine(t.Progressed.Hash(), t.Mu.Hash())
}

type RUnlock struct {
	Progressed defs.Goro
	Mu         loc.Location
}

func (t RUnlock) PrettyPrint() {
	fmt.Println("Read unlocking", t.Mu, "on thread", t.Progressed)
}

func (t RUnlock) String() (str string) {
	return t.Progressed.String() + "-[ RUnlock(" + t.Mu.String() + ") ]"
}

func (t RUnlock) Hash() uint32 {
	return utils.HashCombine(t.Progressed.Hash(), t.Mu.Hash())
}
