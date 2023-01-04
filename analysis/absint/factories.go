package absint

import (
	"github.com/cs-au-dk/goat/analysis/defs"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/hmap"

	"github.com/benbjohnson/immutable"
)

// factory exposes an API for creating abstract configurations and
// superlocation graphs.
type factory struct{}

// Create retrieves a factory for abstract configurations and superlocation
// graphs.
func Create() factory {
	return factory{}
}

// AbsConfiguration creates a fresh abstract configuration.
func (factory) AbsConfiguration() *AbsConfiguration {
	conf := new(AbsConfiguration)
	conf.Init()
	return conf
}

// absConfigurationHasher is a hasher for abstract configurations.
type absConfigurationHasher struct{}

func (absConfigurationHasher) Equal(a, b *AbsConfiguration) bool {
	return a.Equal(b.Superloc)
}

func (absConfigurationHasher) Hash(a *AbsConfiguration) uint32 {
	return a.Hash()
}

var achasher immutable.Hasher[*AbsConfiguration] = absConfigurationHasher{}

// SuperlocGraph creates a new superlocation with the given abstract configuration
// as entry point.
func (factory) SuperlocGraph(entry *AbsConfiguration) SuperlocGraph {
	canon := hmap.NewMap[*AbsConfiguration](utils.HashableHasher[defs.Superloc]())
	canon.Set(entry.Superloc, entry)
	return SuperlocGraph{entry: entry, canon: canon}
}
