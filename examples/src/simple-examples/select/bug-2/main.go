package main

func main() {
	s := struct{
		readwaitc chan struct{}
		done chan int
	}{
		make(chan struct{}),
		make(chan int),
	}
	nc := struct{
		c chan int
	}{ make(chan int) }

	// signal linearizable loop for current notify if it hasn't been already
	select {
	case s.readwaitc <- struct{}{}:
	default:
	}

	// wait for read state notification
	select {
	case <-nc.c:
		return
	/*
	case <-ctx.Done():
		return ctx.Err()
	*/
	case <-s.done:
		return
	}
}
