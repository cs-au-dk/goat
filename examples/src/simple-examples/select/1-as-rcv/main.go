package main

func main() {
	ch := make(chan int)
	select {
	case x := <-ch: //@ analysis(false)
		go func(x int) {}(x)
	}
}
