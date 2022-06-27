package main

func a() {
	func() {
		ch := make(chan int)
		go func() {
			<-ch
		}()
		ch <- 10
	}()
}

func main() {
	a()
}
