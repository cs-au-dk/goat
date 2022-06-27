package main

var ch = make(chan int)

func init() {
	println("EGF")
}

func init() {
	func() {
		println("ASD")
		go func() {
			ch <- 42 //@ analysis(true)
		}()
	}()
}

var recv = func() int { return <-ch }

func main() {
	recv()
}
