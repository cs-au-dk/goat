package location

import (
	"fmt"
	"go/types"
	"log"

	"github.com/benbjohnson/immutable"
	"github.com/cs-au-dk/goat/utils"
	"golang.org/x/tools/go/ssa"
)

// FieldLocation is a complex location denoting the field of a struct,
// or an element in an array or slice. Slice/array element accesses are
// represented by field AINDEX.
type FieldLocation struct {
	Base  Location
	Index int
}

// AINDEX is a symbolic index used to denote the address of an array element, instead of that in a struct field.
// Struct fields are reduced to natural numbers in the SSA representation, so representing array access as -2
// will never be misrepresented as a struct field.
const AINDEX = -2

// NewArrayElementLocation constructs a FieldLocation where the Index represents an element in an array.
func NewArrayElementLocation(base Location) FieldLocation {
	return FieldLocation{
		Base:  base,
		Index: AINDEX,
	}
}

func (l FieldLocation) Hash() uint32 {
	ihasher := immutable.NewHasher(l.Index)
	return utils.HashCombine(l.Base.Hash(), ihasher.Hash(l.Index))
}

func (l FieldLocation) Equal(ol Location) bool {
	o, ok := ol.(FieldLocation)
	return ok && l == o
}

func (l FieldLocation) Position() string {
	return l.Base.Position()
}

func (l FieldLocation) String() string {
	if l.Index == AINDEX {
		return fmt.Sprintf("%s"+colorize.Index("[*]"), l.Base)
	}
	return fmt.Sprintf("%s.("+colorize.Index("%d")+")", l.Base, l.Index)
}

// GetSite is not defined for any FieldLocation.
func (l FieldLocation) GetSite() (ssa.Value, bool) {
	return nil, false
}

// Returns the nesting level of a field location
func (l FieldLocation) NestingLevel() (res int) {
	switch bl := l.Base.(type) {
	case FieldLocation:
		return res + bl.NestingLevel()
	}

	return 0
}

// Type returns the Go type of the field element.
func (l FieldLocation) Type() types.Type {
	ptyp := l.Base.Type()

	// The base has to have either a pointer or a slice type
	switch utyp := ptyp.Underlying().(type) {
	case *types.Pointer:
		ptyp = utyp.Elem()
	case *types.Slice:
		if l.Index != AINDEX {
			log.Fatalln("Field location in array is not indexed at AINDEX?")
		}
		return types.NewPointer(utyp.Elem())
	default:
		log.Fatalln("Unexpected underlying type:", utyp)
	}

	switch ptyp := ptyp.Underlying().(type) {
	case *types.Tuple:
		// Can the ssa ever construct a pointer to a tuple field?
		// Isn't ssa.Extract always used for tuples?
		return types.NewPointer(ptyp.At(l.Index).Type())
	case *types.Struct:
		return types.NewPointer(ptyp.Field(l.Index).Type())
	case *types.Array:
		if l.Index != AINDEX {
			log.Fatalln("Field location in array is not indexed at AINDEX?")
		}
		return types.NewPointer(ptyp.Elem())
	default:
		panic(fmt.Sprintf("Field location %s base has type %s", l, ptyp))
	}
}
