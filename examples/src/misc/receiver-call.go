package main

type X interface {
	a(int, int)
}

type Y struct{}

func (*Y) a(x int, y int) {

}
