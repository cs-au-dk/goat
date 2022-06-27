package main

func a(ch chan int) {
	go func() {
		ch <- 10
	}()
}

func main() {
	ch1 := make(chan int)
	ch2 := make(chan int)

	go func() {
		<-ch1
		a(ch2)
		a(ch2)
	}()
	<-ch2
}
