package main


func main() {
	ch1 := make(chan int)
	ch2 := make(chan int)

	go func() {
		select { //@ analysis(true)
		case ch1 <- 10:
			println("ASD")
		case <-ch2:
			println("DEF")
		case <-ch2:
			println("HIJ")
		}
	}()

	/*
	go func() {
		<-ch2
	}()
	*/

	select { //@ analysis(true)
	case <-ch1:
	case ch2 <- 10:
	}
}
