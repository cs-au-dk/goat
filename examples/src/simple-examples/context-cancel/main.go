package main

import "context"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	go cancel()
	<-ctx.Done() //@ analysis(true)
	// context.Cancel() is quite complex.
	// Lots of branches that depend on precise handling of heap objects.
}
