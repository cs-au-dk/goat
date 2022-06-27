package main

import "time"

func main() {
	ch := make(chan int)
	go func() {
		defer func() {
			recover()
		}()
		close(ch)
		close(ch)
	}()
	time.Sleep(400000000000)
}
