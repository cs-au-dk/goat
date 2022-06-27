package main

func g() error {
	return nil
}

func main() {
	ch := make(chan string, 1)

	f := func(x string) {
		select {
		case ch <- x:
		default:
		}
		println(make(map[int]int) == nil)
	}

	f("abc")
}
