package set

import (
	"log"
	"testing"
)

func compareResults(T *testing.T, found, expected map[string]struct{}) {

	for str := range expected {
		if _, ok := found[str]; !ok {
			log.Println("Subset", str, "expected but not found")
			T.Fail()
		}
	}

	for str := range found {
		if _, ok := expected[str]; !ok {
			log.Println("Subset", str, "found but not expected")
			T.Fail()
		}
	}

}

func TestSubset(T *testing.T) {
	S := SubsetsV("A", "B", "C", "D")

	expected := map[string]struct{}{
		"":     {},
		"A":    {},
		"B":    {},
		"C":    {},
		"D":    {},
		"AB":   {},
		"AC":   {},
		"AD":   {},
		"BC":   {},
		"BD":   {},
		"CD":   {},
		"ABC":  {},
		"ABD":  {},
		"ACD":  {},
		"BCD":  {},
		"ABCD": {},
	}

	found := make(map[string]struct{})

	S.ForEach(func(i []interface{}) {
		str := ""
		for _, i := range i {
			str += i.(string)
		}
		found[str] = struct{}{}
	})

	compareResults(T, expected, found)
}

func TestEmpty(T *testing.T) {
	S := SubsetsV()

	expected := map[string]struct{}{
		"": {},
	}

	found := make(map[string]struct{})

	S.ForEach(func(i []interface{}) {
		str := ""
		for _, i := range i {
			str += i.(string)
		}
		found[str] = struct{}{}
	})

	compareResults(T, expected, found)
}
