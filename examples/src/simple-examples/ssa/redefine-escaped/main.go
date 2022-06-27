package main


func main() {
	ch := make(chan int)

	if 2 * 2 == 4 {
		ch := make(chan int)
		_ = ch
		println("XD")
	}

	go func() {
		ch <- 10
	}()


	<-ch //@ analysis(true)
}
