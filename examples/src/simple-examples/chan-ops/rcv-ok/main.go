package main

func main() {
	ch := make(chan int)
	go func() { close(ch) }()
	v, ok := <-ch //@ analysis(true)
	println(v, ok)
}
