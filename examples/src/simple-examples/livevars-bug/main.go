package main


func main() {
	ch1 := make(chan int)

	go func() {
		ch1 <- 42 //@ releases
	}()

	ch2 := func() chan int {
		ch2 := make(chan int)

		go func() {
			value := <-ch1 //@ releases
			ch2 <- value //@ releases
		}()

		return ch2
	}()

	for {
		<-ch2 //@ analysis(false)
	}
}
