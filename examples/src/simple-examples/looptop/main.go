package main

func f() bool {
	var b bool
	for range []int{} {
		b = true
	}
	return b
}

func main() {
	f()
}
