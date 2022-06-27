package main

func g(m map[struct{}]struct{ x int }) {
	m[struct{}{}] = struct{ x int }{}
}

func f() {
	m := make(map[struct{}]struct{ x int })
	g(m)
	println(m[struct{}{}].x)

}
func main() {
	f()
}
