package defs

type factory struct{}

func Create() factory {
	return factory{}
}
