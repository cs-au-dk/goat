package main

func main() {
	ch := make(chan bool)
	ch1 := make(chan bool)
	go func() {
		for {
			select { //@ analysis(true)
			case <-ch:
			case ch1 <- true:
			}
		}
	}()

	for {
		select { //@ analysis(true)
		case ch <- true:
		case <-ch1:
		}
	}
}
