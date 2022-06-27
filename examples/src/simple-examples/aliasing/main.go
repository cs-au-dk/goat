package main

func main() {
	ch1 := make(chan int)
	ch2 := make(chan int)
	var ch chan int
	if 2 * 4 == 8 {
		ch = ch1
	} else {
		ch = ch2
	}

	// TODO: Maybe change the test now that we have a debugger frontend

	go func() {
		<-ch //@ analysis(true)
	}()
	go func() {
		select { //@ analysis(true)
		case ch <- 10:
			println("ASD")
		case <-ch:
			println("EGF")
		case ch <- 20:
			println("HIJ")
		}
	}()

	ch2 <- 10 //@ analysis(false)
}
