package main

import "reflect"

func main() {
	ch := make(chan int)
	reflect.ValueOf(ch)
}
