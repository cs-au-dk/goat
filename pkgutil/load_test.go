package pkgutil

import "testing"

func TestLoadWithModule(t *testing.T) {
	if pkgs, err := LoadPackages(LoadConfig{
		GoPath:     "../examples",
		ModulePath: "../examples/src/pkg-with-module",
	}, "unrelated-name/..."); err != nil {
		t.Fatal(err)
	} else if len(pkgs) != 2 {
		t.Errorf("Expected load result to contain 2 packages, got: %s", pkgs)
	}
}

func TestLoadFromGoPath(t *testing.T) {
	if pkgs, err := LoadPackages(LoadConfig{GoPath: "../examples"}, "pkg-with-test/...");
		err != nil {
		t.Fatal(err)
	} else if len(pkgs) != 2 {
		t.Errorf("Expected load result to contain 2 packages, got: %s", pkgs)
	}
}
