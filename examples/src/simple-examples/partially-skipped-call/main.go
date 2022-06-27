package main

func external() bool

func f() bool {
	return true
}

func main() {
	var ptr func() bool = external
	if external() { ptr = f }
	println(ptr())
}
