package main

func f(a **int) {
	**a = 10
}

func g() {
	var a **int
	a = new(*int)
	*a = new(int)
	defer f(a)
}

func main() {
	g()
}
