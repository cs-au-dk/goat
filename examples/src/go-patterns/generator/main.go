package main

func Count(start int, end int) chan int {
    ch := make(chan int)

    go func(ch chan int) {
        for i := start; i <= end ; i++ {
            // Blocks on the operation
            ch <- i
        }

        close(ch)
    }(ch)

    return ch
}

func main() {
	println("No bottles of beer on the wall")

	for i := range Count(1, 99) {
		println("Pass it around, put one up,", i, "bottles of beer on the wall")
		// Pass it around, put one up, 1 bottles of beer on the wall
		// Pass it around, put one up, 2 bottles of beer on the wall
		// ...
		// Pass it around, put one up, 99 bottles of beer on the wall
	}

	println(100, "bottles of beer on the wall")
}
