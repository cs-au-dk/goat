package main

func main() {
	var ch chan int
	ch1 := make(chan int)
	ch2 := make(chan int)
	if func() bool { return true }() {
		ch = ch1
	} else {
		ch = ch2
	}
	go func() { <-ch2 }()
	go func() {
		select {
		case ch <- 10:
		case <-ch2:
		}
	}()
	ch <- 20
}
