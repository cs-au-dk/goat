package main

func a() {
	func() {
		defer func() {
			println(recover())
		}()
		recover()
	}()
}

func main() {
	defer a()
	panic("B")
}
