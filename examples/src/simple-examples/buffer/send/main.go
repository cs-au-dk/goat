package main

func main() {
	ch := make(chan bool, 1)
	ch <- true //@ analysis(true)
}
