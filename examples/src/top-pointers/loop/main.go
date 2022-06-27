package main


func mkInt(chan int) *int {
	return new(int)
}

func f(x *int) {
	for i := 0; i < 10; i++ {
		*x = i
		x = mkInt(nil)
	}
}

func main() {
	f(mkInt(nil))
}
