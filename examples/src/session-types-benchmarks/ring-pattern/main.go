package main

// GoLive: replaced fmt.Println with println

//import "fmt"

func numprocs() int {
	return 10
}

func adder(in <-chan int, out chan<- int) {
	for {
		out <-  // normalization
		(<-in + 1)
	}
}

func main() {
	chOne := make(chan int)
	chOut := chOne
	chIn := chOne
	// GoLive: The call to numprocs() is inlined because we spoof it otherwise.
	// It is important that we _do_ enter the loop at least once to be able
	// to prove that we can send on chOne afterwards.
	// A potential fix is to have a heuristic for analyzing "simple" functions
	// for some definition of simple. Maybe no loops/calls.
	// But without context sensitivity it might also create some unnecessary
	// cycles in the control flow graph.
	for i := 0; i < /* numprocs() */ 10 ; i++ {
		chOut = make(chan int)
		go adder(chIn, chOut)
		chIn = chOut
	}
	chOne <- 0 //@ analysis(true)
	println(<-chOut)
}
