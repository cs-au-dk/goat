package main

func main() {
	ch := make(chan int)
	select { //@ analysis(true)
	case <-ch:
		go func() {}()
	default:
	}
}
