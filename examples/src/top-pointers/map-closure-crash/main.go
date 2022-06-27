package main

func g(m map[struct{}]struct{ f func() }) {
	m[struct{}{}] = struct{ f func() }{func() {}}
}

func f() {
	m := make(map[struct{}]struct{ f func() })
	g(m)
	m[struct{}{}] = struct{ f func() }{func() { println("hi") }}
	m[struct{}{}].f()

}
func main() {
	f()
}
