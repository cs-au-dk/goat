package upfront

import (
	"fmt"
	"go/ast"
	"go/token"
	"math"
	"strconv"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

type _LOOP_TYPE int

const (
	_NOT_UNROLLABLE _LOOP_TYPE = iota
	_INCREMENTING_LOOP
	_DECREMENTING_LOOP
)

type unroll struct {
	i *ast.Object
	// Notes whether the loop is safe to unroll
	// The following denote a loop as unsafe:
	// - Side effects on the index in the body of the loop
	// - "goto" statements
	// - "break" and "continue" statements at the level of loop's scope
	// - Calls to non-builtin named functions. No call sensitivty leads to too many inter-procedural edges
	safe bool
}

func (u *unroll) isConst(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Ident:
		o := e.Obj
		if o.Kind != ast.Con {
			return false
		}

		switch o := e.Obj.Decl.(type) {
		case *ast.ValueSpec:
			if len(o.Names) > 1 || len(o.Values) > 1 {
				return false
			}

			return u.isConst(o.Values[0])
		}
	case *ast.UnaryExpr:
		if e.Op == token.SUB {
			return u.isConst(e.X)
		}
	case *ast.BasicLit:
		return e.Kind == token.INT
	}

	return false
}

// Only call when the expression is a guaranteed constant
func (u *unroll) Const(e ast.Expr) int {
	switch e := e.(type) {
	case *ast.Ident:
		decl := e.Obj.Decl.(*ast.ValueSpec)

		for i, x := range decl.Names {
			if x.Obj == e.Obj {
				return u.Const(decl.Values[i])
			}
		}
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			v, err := strconv.Atoi(e.Value)
			if err == nil {
				return v
			}
			// case token.FLOAT:
			// 	v, err := strconv.ParseFloat(e.Value, 64)
			// 	if err != nil {
			// 		return v
			// 	}
		}
	case *ast.UnaryExpr:
		if e.Op == token.SUB {
			return -u.Const(e.X)
		}
	}

	panic(fmt.Sprintf("Attempt to call const on %s : %T failed", e, e))
}

func (u *unroll) Bound(e ast.Expr) int {
	switch e := e.(type) {
	case *ast.BinaryExpr:
		switch {
		case e.Op == token.LEQ || e.Op == token.GEQ:
			if u.isConst(e.Y) {
				return u.Const(e.Y)
			}
			if u.isConst(e.X) {
				return u.Const(e.X)
			}
		case e.Op == token.LSS:
			switch {
			case u.isConst(e.Y):
				return u.Const(e.Y) - 1
				// switch c := c.(type) {
				// case int:
				// 	return c - 1
				// case float64:
				// }
			case u.isConst(e.X):
				return u.Const(e.X) + 1
				// switch c := c.(type) {
				// case int:
				// 	return c + 1
				// case float64:
				// }
			}
		case e.Op == token.GTR:
			switch {
			case u.isConst(e.Y):
				return u.Const(e.Y) + 1
				// switch c := c.(type) {
				// case int:
				// 	return c + 1
				// case float64:
				// }
			case u.isConst(e.X):
				return u.Const(e.X) - 1
				// switch c := c.(type) {
				// case int:
				// case float64:
				// 	return c - 1
				// }
			}
		}
	}

	panic(fmt.Sprintf("Incorrect call to bound on %s", e))
}

// Ensure that an identifier is the loop identifier.
func (u *unroll) isIdent(x ast.Expr) bool {
	switch inc := x.(type) {
	case *ast.Ident:
		return inc.Obj == u.i
	}

	return false
}

type safetyVisitor struct {
	breakInScope    bool
	continueInScope bool
	unroller        *unroll
}

func newSafetyVisitor(u *unroll) *safetyVisitor {
	return &safetyVisitor{true, true, u}
}

func (u *unroll) isUnrollableBody(s *ast.BlockStmt) bool {
	ast.Walk(newSafetyVisitor(u), s)
	return u.safe
}

func (s *safetyVisitor) Visit(n ast.Node) ast.Visitor {
	if s == nil {
		return nil
	}

	u := s.unroller
	if !u.safe {
		return nil
	}

	abort := func() *safetyVisitor {
		u.safe = false
		return nil
	}

	switch n := n.(type) {
	case *ast.BranchStmt:
		switch n.Tok {
		case token.BREAK:
			if s.breakInScope {
				return abort()
			}
		case token.CONTINUE:
			if s.continueInScope {
				return abort()
			}
		case token.GOTO:
			return abort()
		}
	case *ast.SwitchStmt: // Breaks are no longer in scope
		return &safetyVisitor{false, s.continueInScope, u}
	case *ast.ForStmt: // Breaks and continues are no longer in scope
		return &safetyVisitor{false, false, u}
	case *ast.AssignStmt:
		// Check every left-hand side of the assignment
		for _, lhs := range n.Lhs {
			x, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}
			// If the left-hand side of the assignment is performed on the index, abort
			if x.Obj == u.i {
				return abort()
			}
		}
	case *ast.CallExpr:
		switch f := n.Fun.(type) {
		case *ast.FuncLit:
		case *ast.Ident:
			switch f.Name {
			// Builtins are ok
			case "panic":
			case "recover":
			case "new":
			case "make":
			case "len":
			case "cap":
			case "println":
			case "delete":
			case "copy":
			case "close":
			case "append":
			default:
				return abort()
			}
		default:
			return abort()
		}
	case *ast.UnaryExpr:
		// If the unary expression is &i, pessimistically assume it is unsafe and abort
		if x, ok := n.X.(*ast.Ident); n.Op == token.AND && ok && x.Obj == u.i {
			return abort()
		}
	}

	return s
}

func (u *unroll) isUnrollableInit(s ast.Stmt) (unrollable bool, initVal int) {
	switch s := s.(type) {
	case *ast.AssignStmt:
		if s.Tok == token.DEFINE && len(s.Lhs) == 1 && len(s.Rhs) == 1 {
			lhs, rhs := s.Lhs[0], s.Rhs[0]
			if i, ok := lhs.(*ast.Ident); ok && u.isConst(rhs) {
				u.i = i.Obj
				return true, u.Const(rhs)
			}
		}
	case *ast.DeclStmt:
		if d, ok := s.Decl.(*ast.GenDecl); ok {
			if d.Tok == token.VAR && len(d.Specs) == 1 {
				varD := d.Specs[0].(*ast.ValueSpec)

				if len(varD.Names) == 1 && len(varD.Values) == 1 {
					i, rhs := varD.Names[0], varD.Values[0]
					if u.isConst(rhs) {
						u.i = i.Obj
						return true, u.Const(rhs)
					}
				}
			}
		}
	}

	return
}

func (u *unroll) isUnrollableCond(e ast.Expr) (loop _LOOP_TYPE, bound int) {
	switch e := e.(type) {
	case *ast.BinaryExpr:
		switch {
		case e.Op == token.LEQ || e.Op == token.LSS:
			switch {
			case u.isIdent(e.X) && u.isConst(e.Y): // i {<=, <} _
				return _INCREMENTING_LOOP, u.Bound(e)
			case u.isConst(e.X) && u.isIdent(e.Y): // _ {<=, <} i
				return _DECREMENTING_LOOP, u.Bound(e)
			}
		case e.Op == token.GEQ || e.Op == token.GTR:
			switch {
			case u.isIdent(e.X) && u.isConst(e.Y): // i {>=, >} _
				return _DECREMENTING_LOOP, u.Bound(e)
			case u.isConst(e.X) && u.isIdent(e.Y):
				return _INCREMENTING_LOOP, u.Bound(e) // _ {>=, >} i
			}
		}
	}
	return
}

func (u *unroll) isUnrollablePost(s ast.Stmt) (loop _LOOP_TYPE, incr int) {
	switch s := s.(type) {
	case *ast.AssignStmt:
		if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
			return
		}

		lhs := s.Lhs[0]
		rhs := s.Rhs[0]

		// Check that left-hand side is the iterating identifier
		if x, ok := lhs.(*ast.Ident); !ok || x.Obj != u.i {
			return
		}

		switch s.Tok {
		case token.ASSIGN:
			switch e := rhs.(type) {
			case *ast.BinaryExpr:
				switch {
				case e.Op == token.ADD:
					switch {
					case u.isIdent(e.X) && u.isConst(e.Y):
						return _INCREMENTING_LOOP, u.Const(e.Y)
					case u.isConst(e.X) && u.isIdent(e.Y):
						return _INCREMENTING_LOOP, u.Const(e.X)
					}
				case e.Op == token.SUB:
					switch {
					case u.isIdent(e.X) && u.isConst(e.Y):
						return _DECREMENTING_LOOP, u.Const(e.Y)
					case u.isConst(e.X) && u.isIdent(e.Y):
						return _DECREMENTING_LOOP, u.Const(e.X)
					}
				}
			}
		case token.ADD_ASSIGN:
			if u.isConst(rhs) {
				return _INCREMENTING_LOOP, u.Const(rhs)
			}
		case token.SUB_ASSIGN:
			if u.isConst(rhs) {
				return _DECREMENTING_LOOP, u.Const(rhs)
			}
		}
	case *ast.IncDecStmt:
		if x, ok := s.X.(*ast.Ident); !ok || x.Obj != u.i {
			return
		}

		switch s.Tok {
		case token.INC:
			return _INCREMENTING_LOOP, 1
		case token.DEC:
			return _DECREMENTING_LOOP, 1
		}
	}

	return
}

func Transform(c *astutil.Cursor) bool {
	switch s := c.Node().(type) {
	case *ast.ForStmt:
		u := new(unroll)
		u.safe = true

		init_ok, init := u.isUnrollableInit(s.Init)
		cond_ok, bound := u.isUnrollableCond(s.Cond)
		iter_ok, iter := u.isUnrollablePost(s.Post)

		// Check if basic for sub-statements are unrollable
		if !init_ok || cond_ok == _NOT_UNROLLABLE || iter_ok == _NOT_UNROLLABLE {
			return true
		}
		// Check if the sub-statements are consistent in the unrolling strategy
		if cond_ok != iter_ok || (iter_ok == _DECREMENTING_LOOP && bound > init) ||
			(iter_ok == _INCREMENTING_LOOP && bound < init) {
			return true
		}

		if !u.isUnrollableBody(s.Body) {
			return true
		}

		// If at least one run is possible and the iterator is not 0
		diff := int(math.Abs(float64(bound - init)))
		inc := int(math.Abs(float64(iter)))
		if diff*inc >= 0 {
			stmts := []ast.Stmt{s.Init}

			for i := 0; i <= diff/inc; i++ {
				stmts = append(stmts, s.Body)
				stmts = append(stmts, s.Post)
			}

			block := &ast.BlockStmt{
				Lbrace: s.For,
				List:   stmts,
				Rbrace: s.Body.Rbrace,
			}

			c.Replace(block)
		}
	}

	return true
}

// Loop unroller will recursively trasnform the AST,
// by turning for statements of the form:
// for init; guard; iter { S }
// ==>
// init; { S; iter }; ...; { S; iter }
// 			 ---------n times-------------
// where, given "i" as an iteration index,
// "i" is not mutated in the body of the loop, and:
// init - Is an initialization of the form:
// ---- i := c1
// ---- var i := c1, for some constant c1
// guard - Is a boolean expression of the form
// ---- i {<, <=, >=, >} c2
// ---- c2 {<, <=, >=, >} i, for some constant c2
// iter - Is an interation operation of the form
// ---- i{++, --} (equivalent with i = i {+, -} 1)
// ---- i = i {+, -} c3
// ---- i = c3 {+, i} i, for some constant c3
// And "n" is determined by |c2 - c1| / |c3|
func UnrollLoops(pkgs []*packages.Package) []*packages.Package {
	for _, pkg := range pkgs {
		for i, f := range pkg.Syntax {
			pkg.Syntax[i] = astutil.Apply(f, nil, Transform).(*ast.File)
		}
	}

	return pkgs
}
