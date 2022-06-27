package main

import (
	"fmt"
	"sync"
)

func main() {
	wg := sync.WaitGroup{}
	for i := 17; i <= 21; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = fmt.Sprintf("v1.%d", i)
		}()
	}
	wg.Wait()
}
