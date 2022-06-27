package main

type T struct {
	done  chan struct{}
	value int
}

func X(ctx T) {
	ctx.done <- struct{}{} //@ analysis(true)
}

func main() {
	ctx := T{
		done:  make(chan struct{}),
		value: 3,
	}
	go X(ctx)
	<-ctx.done //@ analysis(true)
}
