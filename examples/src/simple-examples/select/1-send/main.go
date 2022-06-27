package main

func main() {
	ch := make(chan int)
	select {
	case ch <- 10: //@ analysis(false)
		go func() {}()
	}
}
