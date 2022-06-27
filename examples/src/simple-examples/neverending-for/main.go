package main

func main() {
	ch1 := make(chan int)
	ch2 := make(chan int)
	var ch chan int
	if (func(b bool) bool { return b })(true) {
		ch = ch1
	} else {
		ch = ch2
	}

	go func() {
		ch2 <- 10
		for {
		}
	}()
	<-ch
}
