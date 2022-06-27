package main

func main() {
	ch := make(chan int)
	select { //@ analysis(true)
	case ch <- 10:
	default:
	}
	go func() {}()
}
