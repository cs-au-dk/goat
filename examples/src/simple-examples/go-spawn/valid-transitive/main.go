package main

func main() {
	ch1 := make(chan int)
	ch2 := make(chan int)

	go func() {
		<-ch2
	}()
	go func() {
		ch2 <- 10
		go func() {
			go func() {
				<-ch1
			}()
		}()
	}()

	ch1 <- 10
}
