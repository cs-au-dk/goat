package main

func closer() func(chan int) {
	return func(ch chan int) {
		close(ch)
	}
}

func main() {
	ch := make(chan int)
	defer closer()(ch)
}
