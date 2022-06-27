package main

import "time"

func f() {
	select {
	case <-make(chan int):
	case <-time.After(100):
	}

}

func main() {
}
