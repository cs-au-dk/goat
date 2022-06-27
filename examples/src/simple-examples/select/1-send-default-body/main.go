package main

func main() {
	ch := make(chan int)
	select { //@ analysis(true)
	case ch <- 10:
		go func() {}()
	default:
		go func() {}()
	}
	go func() {}()
}
