package main

var x = make(chan int)

func main() {
	y := make(chan int)

	go func() {
		y <- 5
	}()
	close(y)
}
