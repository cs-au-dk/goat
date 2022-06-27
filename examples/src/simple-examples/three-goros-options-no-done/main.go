package main

func main() {
	ch := make(chan int)
	done := make(chan interface{})

	// Sender
	go func() {
		ch <- 0
		println("Sent first message")
		ch <- 1
		println("Sent second message")
	}()

	// Contest
	go func() {
		x := <-ch
		x = <-ch
		println("Goro got: ", x)
		done <- struct{}{}
	}()

	x := <-ch
	println("Main got: ", x)

	<-done
}
