package main

import "time"

func main() {
	timer := time.NewTimer(0)
	<-timer.C //@ releases
	<-timer.C //@ blocks
}
