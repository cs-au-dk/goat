package defs

type factory struct{}

// Create is a factory for control locations.
func Create() factory {
	return factory{}
}
