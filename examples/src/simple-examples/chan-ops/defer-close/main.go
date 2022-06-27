package main

func main() {
	ch := make(chan int)
	defer close(ch)
	defer close(ch)
}
