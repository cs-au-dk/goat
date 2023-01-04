package loopinline

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ast/astutil"
)

type continueRewriter ast.Ident

func (s *continueRewriter) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}

	switch n := n.(type) {
	case *ast.BranchStmt:
		// Rewrite continue in scope to break to our label
		// TODO: What do we do with labeled continues?
		if n.Tok == token.CONTINUE && n.Label == nil {
			n.Tok = token.BREAK
			n.Label = (*ast.Ident)(s)
		}
	case *ast.ForStmt: // continues are no longer in scope
		return nil
	case *ast.RangeStmt: // continues are no longer in scope
		return nil
	}

	return s
}

// To avoid problems with nested breaks or continues we do not transform
// loops that have breaks/continues to labels defined outside the loop.
type safetyVisitor struct {
	labels map[string]struct{}
	safe   *bool
}

func newSafetyVisitor() safetyVisitor {
	safe := true
	return safetyVisitor{
		safe: &safe,
	}
}

func (s safetyVisitor) Visit(n ast.Node) ast.Visitor {
	if n == nil || !*s.safe {
		return nil
	}

	switch n := n.(type) {
	case *ast.BranchStmt:
		if n.Label != nil {
			if _, found := s.labels[n.Label.Name]; !found &&
				(n.Tok == token.CONTINUE || n.Tok == token.BREAK) {
				*s.safe = false
				return nil
			}
		}
	case *ast.LabeledStmt:
		newLabels := make(map[string]struct{}, len(s.labels)+1)
		for k, v := range s.labels {
			newLabels[k] = v
		}

		newLabels[n.Label.Name] = struct{}{}
		s.labels = newLabels
		return s
	}

	return s
}

func isUnderscore(e ast.Expr) bool {
	if ident, ok := e.(*ast.Ident); ok {
		return ident.Name == "_"
	} else {
		return false
	}
}

// Transform  "inlines" certain loops, fixing to exactly one iteration.
func Transform(typeinfo map[ast.Expr]types.TypeAndValue) func(*astutil.Cursor) bool {
	cntr := 0
	getIdent := func(prefix string, kind ast.ObjKind) *ast.Ident {
		name := fmt.Sprintf("%s_%d", prefix, cntr)
		cntr++
		return &ast.Ident{Name: name, Obj: ast.NewObj(kind, name)}
	}

	return func(c *astutil.Cursor) bool {
		var body *ast.BlockStmt
		switch s := c.Node().(type) {
		case *ast.ForStmt:
			// If the loop has no condition do not inline it.
			// If the loop has neither a post operation nor initialization,
			// assume it's not iterating over a data structure and do not inline it
			if s.Cond != nil && (s.Post != nil || s.Init != nil) {
				body = s.Body
			}
		case *ast.RangeStmt:
			switch typeinfo[s.X].Type.Underlying().(type) {
			case *types.Chan:
				// We don't do the transformation for range over channel
			case *types.Map:
				// We are not guaranteed to be able to do the lookup on maps
				// because the key type may be unexported to us.
			case *types.Basic:
				// Don't transform ranges over strings
			default:
				body = s.Body

			}
		}

		if body == nil {
			return true
		}

		// Check if we can safely do the transformation
		s := newSafetyVisitor()
		ast.Walk(s, body)
		if !*s.safe {
			return true
		}

		label := getIdent("_INLINE_LOOP_BREAK", ast.Lbl)

		// Rewrite continues to breaks
		ast.Walk((*continueRewriter)(label), body)

		var prefix, postfix []ast.Stmt
		switch s := c.Node().(type) {
		case *ast.ForStmt:
			// Put init and cond before the body for side-effects.
			if s.Init != nil {
				prefix = append(prefix, s.Init)
			}
			prefix = append(prefix, &ast.AssignStmt{
				Lhs: []ast.Expr{&ast.Ident{Name: "_"}},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{s.Cond},
			})
			if s.Post != nil {
				// TODO: Maybe prevent s.Post from running if the loop is broken out of
				postfix = append(postfix, s.Post)
			}
		case *ast.RangeStmt:
			// Assigned the expression to be ranged over to a fresh name so we
			// do not have to evaluate it twice.
			rangedId := getIdent("_INLINE_LOOP_RANGE", ast.Var)
			prefix = append(prefix,
				&ast.AssignStmt{
					Lhs: []ast.Expr{rangedId},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{s.X},
				}, &ast.AssignStmt{
					Lhs: []ast.Expr{&ast.Ident{Name: "_"}},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{rangedId},
				})

			// If the key is non-nil we must handle assignments caused by the range
			if s.Key != nil {
				needVal := s.Value != nil && !isUnderscore(s.Value)
				// Add an assignment to the loop counter
				if !isUnderscore(s.Key) || needVal {
					assTok := s.Tok
					// If the key is an underscore we need to generate a name for it
					// so we can use it in the lookup for the value.
					if isUnderscore(s.Key) {
						s.Key = getIdent("_INLINE_INDEX", ast.Var)
						assTok = token.DEFINE
					}

					prefix = append(prefix, &ast.AssignStmt{
						Lhs: []ast.Expr{s.Key},
						Tok: assTok,
						Rhs: []ast.Expr{
							&ast.BasicLit{
								Kind:  token.INT,
								Value: "0",
							},
						},
					})
				}

				// Add assignment to the value
				if needVal {
					prefix = append(prefix, &ast.AssignStmt{
						Lhs: []ast.Expr{s.Value},
						Tok: s.Tok,
						Rhs: []ast.Expr{
							&ast.IndexExpr{
								X:     rangedId,
								Index: s.Key,
							},
						},
					})
				}
			}
		}

		body.List = append(body.List, &ast.BranchStmt{
			Tok:   token.BREAK,
			Label: label,
		})

		newBlList := append(prefix, &ast.LabeledStmt{
			Label: label,
			Stmt:  &ast.ForStmt{Body: body},
		})
		newBlList = append(newBlList, postfix...)

		c.Replace(&ast.BlockStmt{List: newBlList})

		return true
	}
}
