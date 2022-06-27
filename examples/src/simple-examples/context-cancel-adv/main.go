package main

import (
	"context"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_, _ = context.WithCancel(ctx)
		//cc()
	}()
	go func() {
		<-ctx.Done()
	}()
	for {
		cancel()
	}
}
