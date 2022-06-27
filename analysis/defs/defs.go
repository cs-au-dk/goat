package defs

import (
	u "Goat/utils"

	c "github.com/fatih/color"
)

// For indexing into Value products
type vk = int

var VK = struct {
	ALLOC vk
	PTR   vk
	CH    vk
}{
	ALLOC: 0,
	PTR:   1,
	CH:    2,
}

var colorize = struct {
	Go       func(...interface{}) string
	Superloc func(...interface{}) string
	Panic    func(...interface{}) string
	Index    func(...interface{}) string
}{
	Go: func(is ...interface{}) string {
		return u.CanColorize(c.New(c.FgHiMagenta).SprintFunc())(is...)
	},
	Superloc: func(is ...interface{}) string {
		return u.CanColorize(c.New(c.FgHiBlue).SprintFunc())(is...)
	},
	Panic: func(is ...interface{}) string {
		return u.CanColorize(c.New(c.FgHiRed).SprintFunc())(is...)
	},
	Index: func(is ...interface{}) string {
		return u.CanColorize(c.New(c.FgHiCyan).SprintFunc())(is...)
	},
}
