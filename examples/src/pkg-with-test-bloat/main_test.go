package main

import "testing"

func TestT(t *testing.T) {
	var x T = 5
	x.PublicMethod()
}
