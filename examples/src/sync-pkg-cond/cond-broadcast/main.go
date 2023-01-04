package main

import (
	"bufio"
	"os"
	"sync"
)

func main() {
	c := sync.NewCond(&sync.Mutex{})

	go func() {
		c.L.Lock() //@ releases
		c.Wait() //@ blocks
		c.L.Unlock()
	}()
	go func() {
		c.L.Lock() //@ releases
		chr, _ := bufio.NewReader(os.Stdin).ReadByte()
		switch chr {
		case 'a':
			c.Wait() //@ blocks
			c.L.Unlock()
		default:
			c.Wait() //@ blocks
			c.L.Unlock()
		}
	}()

	c.Broadcast()
}
