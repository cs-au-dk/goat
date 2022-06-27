package main

func main() {
	select {
	default:
	}
	go func() {}()
}
