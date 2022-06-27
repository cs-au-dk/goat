package main

func events() <-chan int {
	ch := make(chan int)
	go func() {
		for {
			ch <- 1
		}
	}()
	return ch
}

func main() {
	for {
		select {
		case i := <-events():
			println("i=", i)
		}
	}
}
