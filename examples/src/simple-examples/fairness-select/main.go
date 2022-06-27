package main

// "Synchronization fairness" example from paper

func main() {
	ch := make(chan int)
	a := make(chan int)
	b := make(chan int)

	go func() {
		for {
			select { //@ analysis(true)
			case a <- 1:
			case <-b:
				ch <- 10
			}
		}
	}()

	go func() {
		for {
			select { //@ analysis(false)
			case <-a:
			case b <- 1:
			}
		}
	}()


	<-ch //@ analyze(true) // Requires refinement of C (to add a and b)
}
