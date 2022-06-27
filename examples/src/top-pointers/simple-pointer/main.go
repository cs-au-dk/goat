package main

func ch() {
	var a chan int

	if func() bool { return true }() {
		a = make(chan int)
	} else {
		a = make(chan int)
	}
	<-a
}

func array() {
	var a [100]int

	if func() bool { return true }() {
		a = [100]int{}
	} else {
		a = [100]int{}
	}
	_ = a[5]
}

func fun() {
	var a func()

	if func() bool { return true }() {
		a = func() {}
	} else {
		a = func() {}
	}
	a()
}

type I interface {
	Do()
}
type i1 struct{ x int }
type i2 struct{}

func (e i1) Do() { println(e.x) }
func (*i2) Do()  {}

func interf() {
	var a I

	switch func() int { return 1 }() {
	case 1:
		a = i1{}
	case 2:
		a = &i1{}
	case 3:
		a = &i2{}
	}
	a.Do()
}
func main() {
	// ch()
	// array()
	interf()
}
