package main

import impitf "importable-interfaces"

type x int
func (x) F() {}

func main() {
	var i impitf.I_F = struct{ x }{ 10 }
	i.F()
}
