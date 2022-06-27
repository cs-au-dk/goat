package main

import "sync"

func main() {
	var locker sync.Locker = ((*sync.RWMutex)(nil)).RLocker()
	locker.Lock()
}
