package gotopo

import (
	"github.com/cs-au-dk/goat/utils"

	"github.com/fatih/color"
)

var colorize = struct {
	Func    func(...interface{}) string
	ChanSet func(...interface{}) string
	Chan    func(...interface{}) string
	SyncSet func(...interface{}) string
	Sync    func(...interface{}) string
}{
	Func: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgYellow).SprintFunc())(is...)
	},
	ChanSet: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgBlue).SprintFunc())(is...)
	},
	Chan: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgHiBlue).SprintFunc())(is...)
	},
	SyncSet: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgGreen).SprintFunc())(is...)
	},
	Sync: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgHiGreen).SprintFunc())(is...)
	},
}
