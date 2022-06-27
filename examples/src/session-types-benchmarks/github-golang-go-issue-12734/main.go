package main

// This is a bug report from golang/go which the deadlock detector does not
// detect a deadlock because the net pacakge is loaded (disables detector).

// GoLive: replaced fmt.Println with println

import (
	//"fmt"
	"net/http"
)

func useless(address string) []byte {
	http.Get("https://www.google.com/")
	return nil
}

func test_a(test_channel chan int) {
	test_channel <- 1
	return
}

func test() {
	test_channel := make(chan int)
	for i := 0; i < 10; i++ {
		go test_a(test_channel)
	}
	for {
		println(<-test_channel)
	}
}
func main() {
	test()
}
