package main

func a(x **interface{}) {
	println(x)
	println(*x)
	println(**x)
}

func main() {
	x := new(*interface{})
	*x = new(interface{})
	**x = struct{}{}
	a(x)
	// _ = new(bool)
	// a(new(bool))
	// a(new(bool))
}
