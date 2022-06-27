package main

func main() {
	ch := make(chan int)
	for range ch {
	}
}
