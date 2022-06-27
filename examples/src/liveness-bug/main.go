package main

func work(ch chan int) {
	for {
		ch <- 1 // @ analysis(true) // Requires refinement of C (to include ch2)
	}
}

func main() {
	ch1 := make(chan int)
	ch2 := make(chan int)
	ch3 := make(chan int)

	go work(ch1)
	go work(ch1)
	go func() {
		ch3 <- 1 //@ analysis(false)
	}()

	for {
		select { //@ analysis(true)
		case <-ch1:
		case <-ch2:
			<-ch3
			return
		}
	}
}
