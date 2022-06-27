package main

import (
	"bufio"
	"os"
	"sync"
)

func main() {
	c := sync.NewCond(&sync.Mutex{})
	for chr, _ := bufio.NewReader(os.Stdin).ReadByte(); chr != 'a'; {
		c = sync.NewCond(&sync.Mutex{})
	}

	go func() {
		c.L.Lock()
		c.Wait()
		c.L.Unlock()
	}()
	go func() {
		c.L.Lock()
		c.Wait()
		c.L.Unlock()
	}()

	c.Broadcast()
}
