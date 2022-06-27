package main

import "time"

func main() {
	ticker := time.NewTicker(1)
	<-ticker.C //@ releases
	<-ticker.C //@ releases
}
