package main

//import "fmt"

func main() {
	chanOwner := func() <-chan int {
		results := make(chan int, 5)
		go func() {
			defer close(results)
			for i := 0; i <= 5; i++ {
				results <- i
			}
		}()
		return results
	}

	consumer := func(results <-chan int) {
	for result := range results {
		println("Received:", result)
	}
	println("Done receiving!")
}

	results := chanOwner()
	consumer(results)
}
