// +build ignore

package defs

import (
	"log"

	"github.com/benbjohnson/immutable"
)

type hasherTemplate struct{}

func (hasherTemplate) cast(elem interface{}) HInterface {
	loc, ok := elem.(HInterface)
	if !ok {
		log.Fatalf("Prettyhashee hasher was given non-Prettyhashee: %T %v\n", elem, elem)
	}
	return loc
}

func (h hasherTemplate) Equal(a, b interface{}) bool {
	return h.cast(a).Equal(h.cast(b))
}

func (h hasherTemplate) Hash(key interface{}) uint32 {
	return h.cast(key).Hash()
}
