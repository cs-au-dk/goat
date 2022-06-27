package main

func g(m map[struct{}]*int) {
	m[struct{}{}] = new(int)
}

func f() {
	m := make(map[struct{}]*int)
	g(m)
	println(*m[struct{}{}])

}
func main() {
	f()
}
