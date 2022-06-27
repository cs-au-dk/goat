package main

func main() {
	ch := make(chan int)
	select { //@ analysis(false)
	case x := <-ch:
		go func(x int) {}(x)
	case x := <-ch:
		go func() int { return x }()
	case x := <-ch:
		go func(x int) {}(x * 2)
	}
}
