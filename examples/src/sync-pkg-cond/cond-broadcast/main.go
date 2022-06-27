package main

import (
	"bufio"
	"os"
	"sync"
)

func main() {
	c := sync.NewCond(&sync.Mutex{})

	go func() {
		c.L.Lock()
		c.Wait()
		c.L.Unlock()
	}()
	go func() {
		c.L.Lock()
		chr, _ := bufio.NewReader(os.Stdin).ReadByte()
		switch chr {
		case 'a':
			c.Wait()
			c.L.Unlock()
		default:
			c.Wait()
			c.L.Unlock()
		}
	}()

	c.Broadcast()
}
