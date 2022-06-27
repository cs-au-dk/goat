package main

func main() {
	ch1 := make(chan int)
	ch2 := make(chan int)

	go func() {
		<-ch1
		for {
			go func() {
				ch2 <- 10
			}()
		}
	}()
	<-ch2
}
