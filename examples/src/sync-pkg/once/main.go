package main

import "sync"

func main() {
	var once1, once2 sync.Once

	ch := make(chan int) //@ chan(ch)

	once1.Do(func() {}) // Dummy to set up a charge

	// A spurious cycle causes the query to fail.
	// Can be solved with modeling or by introducing context sensitivity here.
	// Object sensitivity is sufficient in this case.
	make(chan string, 1) <- "cheating" // @ chan_query(ch, status, true)

	once2.Do(func() {
		close(ch)
	})
}
