package main

func f(x *int) {
	*x = 10
}

func g() {
	f(new(int))
}

func main() {
	g()
}
