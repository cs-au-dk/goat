package absint

import (
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/hmap"
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

var achasher utils.Hasher[*AbsConfiguration] = absConfigurationHasher{}

func (factory) SuperlocGraph(entry *AbsConfiguration) SuperlocGraph {
	canon := hmap.NewMap[*AbsConfiguration](achasher)
	canon.Set(entry, entry)
	return SuperlocGraph{entry: entry, canon: canon}
}
