package lattice

import (
	"go/types"
	"testing"
)

func TestFlatJoin(t *testing.T) {
	v1 := ZeroValueForType(types.Typ[types.Int]).BasicValue()
	v2 := Create().Element().AbstractValue(
		AbstractValueConfig{Basic: 2},
	).BasicValue()

	joined := v1.Join(v2).Flat()

	if !v1.leq(joined) {
		t.Errorf("%s is not smaller than %s", v1, joined)
	}

	if !v2.leq(joined) {
		t.Errorf("%s is not smaller than %s", v2, joined)
	}

	if !joined.IsTop() {
		t.Error("Expected", joined, "to be ⊤")
	}
}
