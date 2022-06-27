package main

type myint int

type s struct{}
func (s) make() *myint {
	x := new(int)
	return (*myint)(x)
}

func test(x *myint) {
	*x = 10
	<- make(chan int) //@ blocks
}

func main() {
	var s s
	test(s.make())
}
