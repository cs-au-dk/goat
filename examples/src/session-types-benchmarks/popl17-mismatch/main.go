package main

// GoLive: replaced fmt.Println with println. replaced call to time.Sleep with irrelevant channel op

//"fmt"
//"time"

func Work() {
	irrelevant := make(chan int)
	for {
		println("Working")
		<-irrelevant
	}
}

func Send(ch chan<- int) { ch <- 42 }
func Recv(ch <-chan int, done chan<- int) {
	done <- // normalization
	<-ch    //@ blocks
}

func main() {
	ch, done := make(chan int), make(chan int)
	go Send(ch)
	go Recv(ch, done)
	go Recv(ch, done)
	go Work()

	<-done // @ analysis(true) // enable when we can refine C
	<-done
}
