package main

// With allocation site abstraction we have to be careful when we
// "guarantee" that channels alias

func C() chan int {
	return make(chan int)
}

func main() {
	go func() {
		C() <- 10
	}()

	<-C() //@ analysis(false)
}
