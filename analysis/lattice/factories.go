package lattice

type factory struct{}

type latticeFactory struct{}

type elementFactory struct{}

var latFact = latticeFactory{}

var elFact = elementFactory{}

var fact = factory{}

func (factory) Lattice() latticeFactory {
	return latFact
}

func (factory) Element() elementFactory {
	return elFact
}

// Factory method to create lattices or lattice elements.
func Create() factory {
	return fact
}

// Element factories
func Elements() elementFactory {
	return elFact
}
