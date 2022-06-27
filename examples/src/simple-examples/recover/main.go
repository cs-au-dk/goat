package main

func main() {
	ch := make(chan int, 1)

	defer func() {
		ch <- 10 //@ releases
	}()

	defer func() {
		if err := recover(); err != nil {
			// Create a new context such that we do not end up joining states after the branch
			go func(){}()
			ch <- 10
		}
	}()
}
