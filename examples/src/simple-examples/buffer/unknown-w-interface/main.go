package main

func main() {
	ch := make(chan interface{}, func() int { return 10 }())

	println(<-ch == nil) //@ blocks
}
