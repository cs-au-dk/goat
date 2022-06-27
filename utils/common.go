package utils

import (
	"bufio"
	"fmt"
	T "go/types"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/fatih/color"
)

func TimeTrack(start time.Time, name string) {
	fmt.Printf("%s took %s\n", name, time.Since(start))
}

func VerbosePrint(format string, a ...interface{}) (n int, err error) {
	if Opts().Verbose() {
		return fmt.Printf(format, a...)
	}
	return 0, nil
}

// Atoi function that fatals instead of returing a tuple with an error
func Atoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Fatalln(err)
	}
	return i
}

// TypeCompat checks whether a value allocated with `allocType` may
// appear in value with declared type `declType`.
// For instance, a pointer to an int allocated with `new(int)` may
// appear in a value with declared type pointer to `myint`, where
// `type myint int`, if the pointer is appropriately casted on the way.
func TypeCompat(declType, allocType T.Type) (res bool) {
	VerbosePrint("Starting equality check\n")
	res = typeCompat(declType, allocType, []*T.Named{})
	if opts.verbose {
		if res {
			fmt.Println("Equality check", color.GreenString("succeded"))
		} else {
			fmt.Println("Equality check", color.RedString("failed"))
		}
	}
	VerbosePrint("\n")
	return res
}

func typeCompat(t1, t2 T.Type, visited []*T.Named) (res bool) {
	if opts.verbose {
		fmt.Printf("Checking equality between: %s and %s\n",
			color.RedString(t1.String()),
			color.BlueString(t2.String()))
	}

	// Shortcut
	if t1 == t2 || T.AssignableTo(t2, t1) {
		return true
	}

	// If the second type is named, but the first isn't
	// resolve the second type to its underlying type
	_, ok1 := t1.(*T.Named)
	_, ok2 := t2.(*T.Named)
	if !ok1 && ok2 {
		return typeCompat(t1, t2.Underlying(), visited)
	}

	// If either of the types is an interface,
	// optimistically assume that they may be of the same type
	_, ok1 = t1.(*T.Interface)
	_, ok2 = t2.(*T.Interface)
	if ok1 || ok2 {
		return true
	}

	switch t1 := t1.(type) {
	case *T.Named:
		switch t2 := t2.(type) {
		case *T.Named:
			// If both types are named the same,
			// and in the same package, they are equal
			if t1.Obj().Pkg() == t2.Obj().Pkg() &&
				t1.Obj().Name() == t2.Obj().Name() {
				return true
			}

			// Avoid cyclical checks
			var t1found, t2found bool
			for _, t := range visited {
				if t == t1 {
					t1found = true
				}
				if t == t2 {
					t2found = true
				}
			}

			if t1found && t2found {
				return false
			}

			visited = append(visited, []*T.Named{t1, t2}...)
			return typeCompat(t1.Underlying(), t2.Underlying(), visited)
		}

		// First type is named but second isn't
		return typeCompat(t1.Underlying(), t2, visited)
	case *T.Array:
		switch t2 := t2.(type) {
		case *T.Array:
			return t1.Len() == t2.Len() && typeCompat(t1.Elem(), t2.Elem(), visited)
		}
	case *T.Basic:
		switch t2 := t2.(type) {
		case *T.Basic:
			return t1.Kind() == t2.Kind()
		}
	case *T.Chan:
		switch t2 := t2.(type) {
		case *T.Chan:
			return typeCompat(t1.Elem(), t2.Elem(), visited)
		}
	case *T.Map:
		switch t2 := t2.(type) {
		case *T.Map:
			return typeCompat(t1.Key(), t2.Key(), visited) && typeCompat(t1.Elem(), t2.Elem(), visited)
		}
	case *T.Pointer:
		switch t2 := t2.(type) {
		case *T.Pointer:
			return typeCompat(t1.Elem(), t2.Elem(), visited)
		case *T.Slice:
			switch t1 := t1.Elem().Underlying().(type) {
			case *T.Array:
				return typeCompat(t1.Elem(), t2.Elem(), visited)
			}
		}
	case *T.Signature:
		switch t2 := t2.(type) {
		case *T.Signature:
			return typeCompat(t1.Params(), t2.Params(), visited) &&
				typeCompat(t1.Results(), t2.Results(), visited)
		}
	case *T.Slice:
		switch t2 := t2.(type) {
		case *T.Slice:
			return typeCompat(t1.Elem(), t2.Elem(), visited)
		case *T.Pointer:
			switch t2 := t2.Elem().Underlying().(type) {
			case *T.Array:
				return typeCompat(t1.Elem(), t2.Elem(), visited)
			}
		}
	case *T.Struct:
		switch t2 := t2.(type) {
		case *T.Struct:
			if t1.NumFields() != t2.NumFields() {
				return false
			}
			for i := 0; i < t1.NumFields(); i++ {
				if t1.Tag(i) != t2.Tag(i) || !typeCompat(t1.Field(i).Type(), t2.Field(i).Type(), visited) {
					return false
				}
			}
			return true
		}
	case *T.Tuple:
		switch t2 := t2.(type) {
		case *T.Tuple:
			if t1.Len() != t2.Len() {
				return false
			}
			for i := 0; i < t1.Len(); i++ {
				if !typeCompat(t1.At(i).Type(), t2.At(i).Type(), visited) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func Prompt() {
	bufio.NewReader(os.Stdin).ReadString('\n')
}
