package main

import (
	"os"
	"os/signal"
	"syscall"
)

func f(ch chan os.Signal) {
	ch2 := make(chan os.Signal)
	signal.Notify(ch2, syscall.SIGSTOP)
	<-ch2
	<-ch
}

func main() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGSTOP)
	f(ch)
	<-ch
}
