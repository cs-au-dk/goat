package main

func main() {
	ch1 := make(chan bool)
	ch2 := make(chan bool)
	go func(x int) {
		<-ch1
		if x == 30 {
			ch1 <- true
		} else {
			<-ch2
		}
	}(10)

	x := 10
	ch1 <- true
	if x == 30 {
		ch1 <- true
	} else {
		<-ch2
	}
}
