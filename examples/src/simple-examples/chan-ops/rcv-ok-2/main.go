package main

func main() {
	ch := make(chan int)

	go func() {
		ch <- 10
	}()

	go func() {
		_, ok := <-ch
		_, ok = <-ch
		println(ok)
	}()


	ch <- 20 //@ analysis(true)
}
