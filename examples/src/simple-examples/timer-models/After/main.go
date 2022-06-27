package main

import "time"

func main() {
	c := time.After(0)
	t := <-c //@ releases
	println(t.Unix())

	// Make sure that we do not use the same model for the instance method
	ok := t.After(time.Now())
	if !ok {
		println("Yes")
	}

	<-c //@ blocks
}
