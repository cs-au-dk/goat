package main

func main() {
	ch := make(chan int)

	select {
	case <-ch:
		println("A")
	case ch <- 10:
		println("B")
	case <-ch:
		println("C")
	case ch <- 20:
		println("D")
	}
}
