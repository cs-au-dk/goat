package loopinline

import (
	"go/printer"
	"strings"
	"testing"

	"github.com/cs-au-dk/goat/pkgutil"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestLoopInline(t *testing.T) {
	tests := []struct{ name, src string }{
		{"Basic", `
			package main
			func main() {
				for i := 0; i < 10; i++ {
					println(i)
				}
			} `,
		},
		{"Reassign_i", `
			package main

			func main() {
				var sl [10]int
				for i := range sl {
					i := i
					println(i)
				}
			}`,
		},
		{"Assign_i_outside", `
			package main

			func main() {
				x := 255
				i := 0
				for i = 0; x > 127; i++ {
				}
			}`,
		},
		{"Redeclare_i_nonint", `
			package main

			func main() {
				for i := 0; i < 10; i++ {
					var i interface{}
					println(i)
				}
			}`,
		},
		/*
			{"String", `
				package main

				const s = "abc"

				func main() {
					for a, b := range s {
						println(a, b)
					}
				}`,
			},
		*/
		{"Array", `
			package main
			func main() {
				var sl [10]int
				for a, b := range sl {
					println(a, b)
				}
				for _, b := range sl {
					println(b)
				}
				for a, _ := range sl {
					println(a)
				}
				for a := range sl {
					println(a)
				}
				for _ = range sl {
					println()
				}
				for range sl {
					println()
				}

				var a, b int
				for a, b = range sl {
					println(a, b)
				}
				for _, b = range sl {
					println(b)
				}
				for a, _ = range sl {
					println(a)
				}
				for a = range sl {
					println(a)
				}
			} `,
		},
		{"Slice", `
			package main
			func main() {
				sl := make([]int, func() int { return 10 }())
				for a, b := range sl {
					println(a, b)
				}
				for _, b := range sl {
					println(b)
				}
				for a, _ := range sl {
					println(a)
				}
				for a := range sl {
					println(a)
				}
				for _ = range sl {
					println()
				}
				for range sl {
					println()
				}

				var a, b int
				for a, b = range sl {
					println(a, b)
				}
				for _, b = range sl {
					println(b)
				}
				for a, _ = range sl {
					println(a)
				}
				for a = range sl {
					println(a)
				}
			} `,
		},
		{"Continue", `
			package main
			func main() {
				var sl [10]int
				for a, b := range sl {
					if a > 5 {
						continue
					}
					println(a, b)
				}

				for i := 0; i < 10; i++ {
					if i < 5 { continue }
				}

				OUT:
				for i := 0; i < 10; i++ {
					for j := 0; j < 10; j++ {
						println(j)
						break OUT
					}
				}
			}`,
		},
		{"Non_iterating_for", `
			package main
			func main() {
				var b bool
				for !b {
					b = !b
				}					
			}`,
		},
	}

	setup := func(t *testing.T, src string) ([]*packages.Package, string) {
		pkgs, err := pkgutil.LoadPackagesFromSource(src)
		if err != nil {
			t.Fatal("Failed to load package:", err)
		}

		err = InlineLoops(pkgs)
		if err != nil {
			t.Errorf("Error occurred during inlining %v", err)
		}

		var buf strings.Builder
		pkg := pkgs[0]
		printer.Fprint(&buf, pkg.Fset, pkg.Syntax[0])

		pp := buf.String()
		t.Log(pp)
		return pkgs, pp
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pkgs, pp := setup(t, test.src)

			if strings.Contains(pp, "range") {
				t.Error("Transformed source still contains a range")
			}

			if strings.Contains(pp, "continue") {
				t.Error("Transformed source still contains a continue")
			}

			prog, _ := ssautil.AllPackages(pkgs, ssa.SanityCheckFunctions|ssa.BuildSerially)
			prog.Build()
		})
	}

	t.Run("range-sideeffects", func(t *testing.T) {
		_, pp := setup(t, `package main
			func f() []int {
				return []int{1, 2, 3}
			}

			func main() {
				for i, x := range f() {
					println(i, x)
				}
			}`)

		if strings.Count(pp, "f()") != 2 {
			t.Error("Too many invokes to f()")
		}
	})

	t.Run("pkg-with-test", func(t *testing.T) {
		pkgs, err := pkgutil.LoadPackages(pkgutil.LoadConfig{
			GoPath:       "../../../examples",
			ModulePath:   "../../../examples/src/pkg-with-test",
			IncludeTests: true,
		}, "pkg-with-test")
		if err != nil {
			t.Fatal(err)
		}

		err = InlineLoops(pkgs)

		for _, pkg := range pkgs {
			for _, file := range pkg.Syntax {
				var buf strings.Builder
				printer.Fprint(&buf, pkg.Fset, file)
				t.Log(buf.String())
			}
		}

		if err != nil {
			t.Fatal(err)
		}

		prog, _ := ssautil.AllPackages(pkgs, ssa.SanityCheckFunctions|ssa.BuildSerially)
		prog.Build()
	})
}
