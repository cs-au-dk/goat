package main

type ChStruct struct {
	x       *int
	payload chan int
}

func (obj *ChStruct) irrelv() {
	obj.x = new(int)
}

func f() {
	x := &ChStruct{
		x:       new(int),
		payload: make(chan int, 1),
	}
	x.irrelv()
	<-x.payload //@ blocks

}

func main() {
	f()
}
