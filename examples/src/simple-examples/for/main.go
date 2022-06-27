package main

func f() chan int {
	var ch chan int
	for i := 0; i < 2; i++ {
		ch = make(chan int)
	}
	go func() {
		<-ch
	}()
	return ch
}

func main() {
	f()
}
