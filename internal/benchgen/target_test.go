package benchgen

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSupportsLoop(t *testing.T) {
	for version, want := range map[string]bool{"go1.23.9": false, "go1.24.0": true, "go1.25rc1": true, "devel go1.26-abc": true, "go2.0": true, "unknown": false} {
		if supportsLoop(version) != want {
			t.Errorf("version %s", version)
		}
	}
}

func TestResolveTarget(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/target\n\ngo 1.24\n")
	write("target.go", `package target
 type Named string
 type Alias = string
 func hidden(s string) (int,error) {return len(s),nil}
 func bytes(b []byte) {}

 func number(n int64) {}
 func flag(v bool) {}
 func ratio(v float32) {}
 func runeArg(v rune) {}

 func tuple(s string, n int64, ok bool) {}
 func pointer(p uintptr) {}
 func alias(a Alias) {}
 func named(n Named) {}
 func generic[T any](s string) {}
 func variadic(s ...string) {}
 func zero() {}
 type T struct{}
 func (T) method(s string) {}
 `)
	for _, name := range []string{"hidden", "bytes", "alias", "number", "flag", "ratio", "runeArg", "tuple", "pointer", "named", "generic", "variadic", "zero", "method", "missing"} {
		t.Run(name, func(t *testing.T) {
			target, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: name})
			wantOK := name == "hidden" || name == "bytes" || name == "alias" || name == "number" || name == "flag" || name == "ratio" || name == "runeArg" || name == "tuple"
			if (err == nil) != wantOK {
				t.Fatalf("target %+v, error %v", target, err)
			}
			if wantOK && target.FuzzName != "FuzzProfadvisor_"+name {
				t.Fatal("wrong name")
			}
			if name == "tuple" && !reflect.DeepEqual(target.InputTypes, []string{"string", "int64", "bool"}) {
				t.Fatalf("types=%q", target.InputTypes)
			}
		})
	}
	write("conflict_test.go", "package target\nimport \"testing\"\nfunc FuzzProfadvisor_hidden(f *testing.F) {}\n")
	if _, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "hidden"}); err == nil {
		t.Fatal("accepted existing symbol")
	}
}

func TestResolveTargetTestFileConflicts(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/target\n\ngo 1.24\n")
	write("target.go", `package target
	 func hidden(s string) (int,error) {return len(s),nil}
	 `)

	// Test 1: _test.go declaring profadvisorTesting must fail
	t.Run("profadvisorTesting in test file", func(t *testing.T) {
		write("conflict1_test.go", "package target\nvar profadvisorTesting string\n")
		_, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "hidden"})
		if err == nil {
			t.Fatal("expected error for profadvisorTesting in test file, got nil")
		}
		if !strings.Contains(err.Error(), "profadvisorTesting") {
			t.Fatalf("error should mention profadvisorTesting, got: %v", err)
		}
	})
	os.Remove(filepath.Join(dir, "conflict1_test.go"))

	// Test 2: _test.go declaring profadvisorMath must fail
	t.Run("profadvisorMath in test file", func(t *testing.T) {
		write("conflict2_test.go", "package target\nvar profadvisorMath int\n")
		_, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "hidden"})
		if err == nil {
			t.Fatal("expected error for profadvisorMath in test file, got nil")
		}
		if !strings.Contains(err.Error(), "profadvisorMath") {
			t.Fatalf("error should mention profadvisorMath, got: %v", err)
		}
	})
	os.Remove(filepath.Join(dir, "conflict2_test.go"))

	// Test 3: _test.go shadowing a predeclared identifier must fail
	t.Run("shadow predeclared string in test file", func(t *testing.T) {
		write("conflict3_test.go", "package target\ntype string struct{}\n")
		_, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "hidden"})
		if err == nil {
			t.Fatal("expected error for shadowing predeclared string in test file, got nil")
		}
		if !strings.Contains(err.Error(), "string") {
			t.Fatalf("error should mention string, got: %v", err)
		}
	})
	os.Remove(filepath.Join(dir, "conflict3_test.go"))

	// Test 4: Control case - ordinary _test.go must succeed
	t.Run("ordinary test file succeeds", func(t *testing.T) {
		write("normal_test.go", "package target\nimport \"testing\"\nfunc TestSomething(t *testing.T) {}\n")
		_, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "hidden"})
		if err != nil {
			t.Fatalf("expected no error for ordinary test file, got: %v", err)
		}
	})
}
