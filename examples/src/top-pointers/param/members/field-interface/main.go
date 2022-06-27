package main

type A struct {
	// Interface type is used to satisfy the utils.TypeEq check
	// between struct{x interface{}} and interface{}.
	x interface{}
}

func f(x *interface{}, a *A) {
	// Crash due to trying to allocate a top value for interface{} where a
	// struct is stored before (because we do not distinguish between
	// fields of a struct and the struct itself).
}

func main() {
	var a A
	f(&a.x, &a)
}
