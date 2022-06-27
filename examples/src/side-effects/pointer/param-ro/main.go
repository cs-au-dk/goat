package main

func f(x *int) {
	println(*x)
}

func g() {
	f(new(int))
}

func main() {
	g()
}
