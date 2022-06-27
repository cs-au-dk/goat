// Command conditional-recur has a recursion with conditional on one goroutine
// and another receiving until a done message is received.
package main

// GoLive: replaced fmt.Println with println

//import "fmt"

func x(ch chan int, done chan struct{}) {
	i := 0
	for {
		if i < 3 {
			ch <- i
			println("Sent", i)
			i++
		} else {
			done <- struct{}{}
			return
		}
	}
}

func main() {
	done := make(chan struct{})
	ch := make(chan int)
	go x(ch, done)
FINISH:
	for {
		select { //@ analysis(true)
		case x := <-ch:
			println(x)
		case <-done:
			break FINISH
		}
	}
}
