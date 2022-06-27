package main

//"fmt"
//"time"

func Work() {
	irrelevant := make(chan int)
	for {
		println("Working")
		//time.Sleep(1 * time.Second)
		<-irrelevant //@ blocks
	}
}

func Send(ch chan<- int) { ch <- 42 }
func Recv(ch <-chan int, done chan<- int) {
	done <- // normalization
	<-ch    //@ analysis(true)
}

func main() {
	ch, done := make(chan int), make(chan int)
	go Send(ch)
	go Recv(ch, done)
	go Work()

	<-done // @ analysis(true) // enable when we can refine C
}
