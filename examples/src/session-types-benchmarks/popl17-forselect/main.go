// Command nodet-for-select is a for-select pattern between two compatible
// recursive select.
package main

// GoLive: replaced fmt.Println with println

//import "fmt"

func sel1(ch1, ch2 chan int, done chan struct{}) {
	for {
		select { //@ analysis(true)
		case <-ch1:
			println("sel1: recv")
			done <- struct{}{}
			return
		case ch2 <- 1:
			println("sel1: send")
		}
	}
}

func sel2(ch1, ch2 chan int, done chan struct{}) {
	for {
		select { //@ analysis(true)
		case <-ch2:
			println("sel2: recv")
		case ch1 <- 2:
			println("sel2: send")
			done <- struct{}{}
			return
		}
	}
}

func main() {
	done := make(chan struct{})
	a := make(chan int)
	b := make(chan int)
	go sel1(a, b, done)
	go sel2(a, b, done)

	<-done //@ analysis(true) // requires refinement of C
	<-done
}
