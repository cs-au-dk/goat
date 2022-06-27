package main

func a() {}
func b() {
	go a()
}
func c() {
	go a()
}

func init() {
	// fmt.Println("main init")
}

var d = (func() func() {
	switch {
	case true:
		return c
	case true:
		return a
	default:
		return b
	}
})()

func main() {
	var a func()
	if true {
		a = b
	} else {
		a = c
	}
	go func() {

	}()
	switch {
	case true:
		defer b()
		defer a()
	case true:
		defer b()
		return
	}
	d()
}
