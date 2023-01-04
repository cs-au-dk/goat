package condinline

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ast/astutil"
)

// Transform inlines calls to sync.NewCond (detected with syntactic analysis only),
// by rewriting them as the allocation of a Cond object.
//
// Example:
//
//	sync.NewCond(lock) ==> &sync.Cond{ L: lock }
func Transform(typeinfo map[ast.Expr]types.TypeAndValue) func(*astutil.Cursor) bool {
	return func(c *astutil.Cursor) bool {
		call, ok := c.Node().(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}

		retType, ok := typeinfo[call].Type.(*types.Pointer)
		if !ok || !utils.IsNamedTypeStrict(retType.Elem(), "sync", "Cond") {
			return true
		}

		newCondSelector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || newCondSelector.Sel.Name != "NewCond" {
			return true
		}

		syncIdent, ok := newCondSelector.X.(*ast.Ident)
		if !ok || syncIdent.Name != "sync" {
			return true
		}

		arg := call.Args[0]

		c.Replace(
			&ast.UnaryExpr{
				OpPos: call.Pos(),
				Op:    token.AND,
				X: &ast.CompositeLit{
					Type:       &ast.SelectorExpr{X: syncIdent, Sel: ast.NewIdent("Cond")},
					Lbrace:     call.Lparen,
					Rbrace:     call.Rparen,
					Incomplete: false,
					Elts: []ast.Expr{
						&ast.KeyValueExpr{
							Key:   ast.NewIdent("L"),
							Colon: arg.Pos(),
							Value: arg,
						},
					},
				},
			},
		)

		return true
	}
}
