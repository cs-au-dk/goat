package absint

import (
	"github.com/cs-au-dk/goat/analysis/defs"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/hmap"

	"github.com/benbjohnson/immutable"
)

type factory struct{}

func Create() factory {
	return factory{}
}

func (factory) AbsConfiguration(lvl ABSTRACTION_LEVEL) (conf *AbsConfiguration) {
	conf = new(AbsConfiguration)
	conf.Init(lvl)
	return
}

type absConfigurationHasher struct{}

func (absConfigurationHasher) Equal(a, b *AbsConfiguration) bool {
	return a.Superlocation().Equal(b.Superlocation())
}

func (absConfigurationHasher) Hash(a *AbsConfiguration) uint32 {
	return a.Hash()
}

var achasher immutable.Hasher[*AbsConfiguration] = absConfigurationHasher{}

func (factory) SuperlocGraph(entry *AbsConfiguration) SuperlocGraph {
	canon := hmap.NewMap[*AbsConfiguration](utils.HashableHasher[defs.Superloc]())
	canon.Set(entry.superloc, entry)
	return SuperlocGraph{entry: entry, canon: canon}
}
