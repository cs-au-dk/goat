package main

func b() {
	go b()
}

func a() {
	go b()
}

func main() {
	go a()
	a()
}
