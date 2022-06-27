package upfront_test

import (
	"Goat/testutil"
	"go/types"
	"testing"
)

func TestWrittenFieldsAnalysis(t *testing.T) {
	type checkFun = func(function, typ, field string)
	setup := func(t *testing.T, src string) (checkWritten, checkNotWritten checkFun) {
		loadRes := testutil.LoadPackageFromSource(t, "testpackage", src)

		check := func(function, typ, field string, written bool) {
			t.Helper()

			pkg := loadRes.Mains[0]

			fun := pkg.Func(function)
			structT := pkg.Type(typ).Type().Underlying().(*types.Struct)

			for fi := 0; fi < structT.NumFields(); fi++ {
				if structT.Field(fi).Name() == field {
					isWritten := loadRes.WrittenFields.IsFieldWrittenFromFunction(fun, structT, fi)
					if isWritten != written {
						t.Errorf("Written(%s, %s, %s) = %v, expected %v", function, typ, field, isWritten, written)
					}
					return
				}
			}

			t.Fatalf("Did not find field %s on %v", field, structT)
		}

		return func(function, typ, field string) {
				check(function, typ, field, true)
			}, func(function, typ, field string) {
				check(function, typ, field, false)
			}
	}

	t.Run("basic", func(t *testing.T) {
		checkWritten, checkNotWritten := setup(t, `
			package main
			type A struct { x, y int }
			type B struct { z int }

			func f(a *A) {
				a.x = 5
				g(a)
			}

			func g(a *A) { a.y = 10 }
			func h(b *B) { }

			func main() {
				a := &A{}
				b := &B{}
				f(a)
				h(b)
			}`,
		)

		checkWritten("f", "A", "x")
		checkWritten("f", "A", "y")

		checkNotWritten("g", "A", "x")
		checkWritten("g", "A", "y")

		checkWritten("main", "A", "x")
		checkWritten("main", "A", "y")

		checkNotWritten("main", "B", "z")
	})

	t.Run("init-has-no-sideeffect", func(t *testing.T) {
		_, checkNotWritten := setup(t, `
			package main
			type B struct { z int }
			func main() {
				b := &B{ z: 10 }
				_ = b

				var b2 B
				b2.z = 5
			}`,
		)

		checkNotWritten("main", "B", "z")
	})

	t.Run("ignore-sideeffect-soundness", func(t *testing.T) {
		checkWritten, checkNotWritten := setup(t, `
			package main
			type A struct { x, y int }

			func f(a *A) *A {
				if a != nil { a.x = 10 }
				return &A{ y: 20 }
			}

			func main() {
				f(f(nil))
			}`,
		)

		checkWritten("f", "A", "x")
		checkNotWritten("f", "A", "y")
	})

	t.Run("field-ptr-behaviour", func(t *testing.T) {
		checkWritten, checkNotWritten := setup(t, `
			package main
			type A struct { x, y int }
			type B struct { a A }

			func f(x *int) { *x = 10 }
			func g(a *A) { *a = A{ x: 5, y: 10 } }
			func h(a *A) { *a = A{ x: 5, y: 10 } }

			func main() {
				var a A
				var b B
				f(&a.x)
				g(&b.a)
				h(&b.a); h(&a)
			}`,
		)

		checkWritten("f", "A", "x")
		checkNotWritten("f", "A", "y")

		checkWritten("g", "B", "a")

		// g does not write to x & y because a is always a field pointer inside
		// B, so that field is said to be written instead.
		// TODO: This means we should use the underlying allocation site when
		// checking for potential writes in the abstract interpreter.
		checkNotWritten("g", "A", "x")
		checkNotWritten("g", "A", "y")

		// h writes to both a and x & y because we also call the function with
		// a non-field pointer.
		checkWritten("h", "B", "a")
		checkWritten("h", "A", "x")
		checkWritten("h", "A", "y")
	})

	t.Run("more-plain-overwrites", func(t *testing.T) {
		checkWritten, _ := setup(t, `
			package main
			type A struct { x, y int }
			type B struct { z int }

			func f(a *A) { *a = A{ x: 5 } }
			func g(b *B) { *b = B{} }

			func main() {
				var a A
				f(&a)
				b := &B{}
				g(b)
			}`,
		)

		checkWritten("f", "A", "x")
		checkWritten("f", "A", "y")

		checkWritten("g", "B", "z")
	})

	t.Run("slices", func(t *testing.T) {
		checkWritten, checkNotWritten := setup(t, `
			package main
			type A struct { x, y int }

			func f(l []A) { l[0].y = 10 }
			func g(l []A) {
				var a *A = &l[0]
				a.y = 5
			}
			func h(a *A) { a.x = 5 }
			func i(a *A) { a.x = 6 }
			func main() {
				l := make([]A, func() int { return 10 }())
				var l2 [2]A
				f(l)
				g(l)

				var a A
				h(&l[0]); h(&l2[0])
				i(&l[0]); i(&a)
			}`,
		)

		checkNotWritten("f", "A", "x")
		checkNotWritten("f", "A", "y")
		checkNotWritten("g", "A", "x")
		checkNotWritten("g", "A", "y")

		checkNotWritten("h", "A", "x")
		checkWritten("i", "A", "x")
	})
}
