package main

type SuperInterface interface {
	f()
}

type SubInterface interface {
	SuperInterface
	g()
}

type impl struct { }
func (impl) f() {}
func (impl) g() {}

type impl2 struct { }
func (impl2) f() {}

var cond bool

// To avoid an entrypoint in these functions
type s struct {}
func (s) make() SuperInterface {
	if cond {
		return impl2{}
	}
	return impl{}
}

func (s s) getSub() SubInterface {
	sup := s.make()
	sub, ok := sup.(SubInterface)
	if !ok {
		panic("???")
	}
	return sub
}

func test(sub SubInterface) {
	sub.g()
	<-make(chan int) //@ blocks
}

func main() {
	var s s
	test(s.getSub())
}
