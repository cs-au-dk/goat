package main

func main() {
	ch := make(chan int)

	go func() {
		<-ch
		for {
		}
	}()
	go func() {
		ch <- 10
	}()

	ch <- 10
}
