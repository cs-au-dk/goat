package main

func main() {
	ch := make(chan int)
	select { //@ analysis(true)
	case x, ok := <-ch:
		go func(x int, ok bool) {}(x, ok)
	default:
	}

	<-make(chan int) //@ blocks
}
