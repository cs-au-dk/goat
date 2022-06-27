package main

func main() {
	ch := make(chan int)
	select { //@ analysis(true)
	case x := <-ch:
		go func(x int) {}(x)
	default:
		go func() {}()
	}
}
