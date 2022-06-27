package main

import "sync/atomic"

func main() {
	c := make(chan int) //@ chan(ch)

	var a atomic.Value
	a.Store(c)
	c2 := a.Load().(chan int)
	close(c2)

	c <- 10 //@ chan_query(ch, status, false)
}
