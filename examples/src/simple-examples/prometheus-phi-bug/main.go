package main

func main() {
	done := make(chan error, 1)
	go func() {
		done <- nil
	}()

	timeout := make(chan int, 1)
	go func() {
		timeout <- 1
	}()

	var stoppedErr error

	select {
	case stoppedErr = <-done:
	case <-done:
	}

	println(stoppedErr)

	stoppedErr = nil

	select {
	case stoppedErr = <-done:
	default:
	}

	stoppedErr = nil
	select {
	case <-done:
	case <-done:
	default:
	}

	println(stoppedErr)
}
