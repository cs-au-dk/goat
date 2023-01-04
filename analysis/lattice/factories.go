package lattice

type (
	// factory a structure that implements methods from which to access
	// the lattice and lattice element factories.
	factory struct{}

	// latticeFactory is a structure that implements methods for creating lattices.
	latticeFactory struct{}

	// elementFactory is a structure that implements methods for creating lattice elements.
	elementFactory struct{}
)

var (
	// latFact is a singleton instantiation of the lattice factory.
	latFact = latticeFactory{}
	// elementFactory is a singleton instantiation of the lattice element factory.
	elFact = elementFactory{}
)

// Lattice gives access to the lattice factory.
func (factory) Lattice() latticeFactory {
	return latFact
}

// Element gives access to the lattice factory.
func (factory) Element() elementFactory {
	return elFact
}

// Create returns a factory for which the methods are used
// to create lattices or lattice elements.
func Create() factory {
	return factory{}
}

// Elements is a short returns a factory for which the methods are used
// to create lattices or lattice elements.
func Elements() elementFactory {
	return elementFactory{}
}
