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
 type Nested struct { Count int64; Ready bool }
 type Input struct { Name string; Nested Nested }
 type BadInput struct { Values []string }
	 type Empty struct{}
 func structured(in Input, suffix string) {}
 func badStructured(in BadInput) {}
	 func empty(in Empty) {}
 func pointer(p uintptr) {}
 func alias(a Alias) {}
 func named(n Named) {}
 func generic[T any](s string) {}
 func variadic(s ...string) {}
 func zero() {}
 type T struct{}
 func (T) method(s string) {}
 `)
	for _, name := range []string{"hidden", "bytes", "alias", "number", "flag", "ratio", "runeArg", "tuple", "structured", "badStructured", "empty", "pointer", "named", "generic", "variadic", "zero", "method", "missing"} {
		t.Run(name, func(t *testing.T) {
			target, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: name})
			wantOK := name == "hidden" || name == "bytes" || name == "alias" || name == "number" || name == "flag" || name == "ratio" || name == "runeArg" || name == "tuple" || name == "structured"
			if (err == nil) != wantOK {
				t.Fatalf("target %+v, error %v", target, err)
			}
			if wantOK && target.FuzzName != "FuzzProfadvisor_"+name {
				t.Fatal("wrong name")
			}
			if name == "tuple" && !reflect.DeepEqual(target.InputTypes, []string{"string", "int64", "bool"}) {
				t.Fatalf("types=%q", target.InputTypes)
			}
			if name == "structured" {
				if !reflect.DeepEqual(target.InputTypes, []string{"string", "int64", "bool", "string"}) {
					t.Fatalf("flattened types=%q", target.InputTypes)
				}
				if !reflect.DeepEqual(target.ArgumentTypes, []string{"Input", "string"}) {
					t.Fatalf("argument types=%q", target.ArgumentTypes)
				}
				if !reflect.DeepEqual(target.ArgumentTemplates, []string{"Input{Name: $0, Nested: Nested{Count: $1, Ready: $2}}", "$3"}) {
					t.Fatalf("argument templates=%q", target.ArgumentTemplates)
				}
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

func TestInterfaceImplementations(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/iface\n\ngo 1.24\n")

	// Test 1: One local implementer, value receiver → succeeds
	t.Run("single value receiver", func(t *testing.T) {
		write("iface.go", `package iface
type Reader interface { Read([]byte) (int, error) }
type MyReader struct { Data []byte; Pos int }
func (mr MyReader) Read(b []byte) (int, error) { return 0, nil }
func useReader(r Reader) {}
`)
		target, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "useReader"})
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if !strings.Contains(target.ArgumentTemplates[0], "MyReader{") {
			t.Fatalf("template should contain MyReader{..., got: %q", target.ArgumentTemplates[0])
		}
		if !strings.Contains(target.ArgumentTemplates[0], "Data: $0") {
			t.Fatalf("template should contain Data: $0, got: %q", target.ArgumentTemplates[0])
		}
		got := target.Implementations
		if !reflect.DeepEqual(got, []string{"Reader=MyReader"}) {
			t.Fatalf("got Implementations=%q, want [\"Reader=MyReader\"]", got)
		}
	})

	// Test 2: Pointer receiver only → succeeds with & wrapper
	t.Run("pointer receiver only", func(t *testing.T) {
		write("iface.go", `package iface
type Reader interface { Read([]byte) (int, error) }
type MyReader struct { Data []byte; Pos int }
func (mr *MyReader) Read(b []byte) (int, error) { return 0, nil }
func useReader(r Reader) {}
`)
		target, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "useReader"})
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if !strings.Contains(target.ArgumentTemplates[0], "&MyReader{") {
			t.Fatalf("template should contain &MyReader{, got: %q", target.ArgumentTemplates[0])
		}
		got := target.Implementations
		if !reflect.DeepEqual(got, []string{"Reader=MyReader"}) {
			t.Fatalf("got Implementations=%q, want [\"Reader=MyReader\"]", got)
		}
	})

	// Test 3: Two implementers → error listing both names
	t.Run("multiple implementers error", func(t *testing.T) {
		write("iface.go", `package iface
type Reader interface { Read([]byte) (int, error) }
type Reader1 struct {}
func (Reader1) Read(b []byte) (int, error) { return 0, nil }
type Reader2 struct {}
func (Reader2) Read(b []byte) (int, error) { return 0, nil }
func useReader(r Reader) {}
`)
		_, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "useReader"})
		if err == nil {
			t.Fatal("expected error for multiple implementers, got nil")
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "Reader1") {
			t.Fatalf("error should mention Reader1, got: %v", errStr)
		}
		if !strings.Contains(errStr, "Reader2") {
			t.Fatalf("error should mention Reader2, got: %v", errStr)
		}
		if !strings.Contains(errStr, "--impl") {
			t.Fatalf("error should suggest --impl, got: %v", errStr)
		}
	})

	// Test 4: Multiple implementers + --impl selecting one → succeeds
	t.Run("multiple implementers with --impl", func(t *testing.T) {
		write("iface.go", `package iface
type Reader interface { Read([]byte) (int, error) }
type Reader1 struct { X string }
func (Reader1) Read(b []byte) (int, error) { return 0, nil }
type Reader2 struct { Y int }
func (Reader2) Read(b []byte) (int, error) { return 0, nil }
func useReader(r Reader) {}
`)
		target, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "useReader", Implementations: []string{"Reader=Reader1"}})
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if !strings.Contains(target.ArgumentTemplates[0], "Reader1{") {
			t.Fatalf("template should contain Reader1{, got: %q", target.ArgumentTemplates[0])
		}
		got := target.Implementations
		if !reflect.DeepEqual(got, []string{"Reader=Reader1"}) {
			t.Fatalf("got Implementations=%q, want [\"Reader=Reader1\"]", got)
		}
	})

	// Test 5: --impl naming a type that does not implement the interface → error
	t.Run("--impl with non-implementing type", func(t *testing.T) {
		write("iface.go", `package iface
type Reader interface { Read([]byte) (int, error) }
type NotAReader struct { X string }
func useReader(r Reader) {}
`)
		_, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "useReader", Implementations: []string{"Reader=NotAReader"}})
		if err == nil {
			t.Fatal("expected error for non-implementing type, got nil")
		}
		if !strings.Contains(err.Error(), "does not implement") {
			t.Fatalf("error should mention does not implement, got: %v", err)
		}
	})

	// Test 6: --impl with malformed syntax → error
	t.Run("malformed --impl syntax", func(t *testing.T) {
		write("iface.go", `package iface
type Reader interface { Read([]byte) (int, error) }
type MyReader struct {}
func (MyReader) Read(b []byte) (int, error) { return 0, nil }
func useReader(r Reader) {}
`)
		_, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "useReader", Implementations: []string{"ReaderMyReader"}})
		if err == nil {
			t.Fatal("expected error for malformed syntax, got nil")
		}
		if !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("error should mention malformed, got: %v", err)
		}
	})

	// Test 7: Parameter of type any → error with zero-method message
	t.Run("any parameter", func(t *testing.T) {
		write("iface.go", `package iface
func useAny(a any) {}
`)
		_, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "useAny"})
		if err == nil {
			t.Fatal("expected error for any parameter, got nil")
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "zero methods") {
			t.Fatalf("error should mention zero methods, got: %v", errStr)
		}
	})

	// Test 8: Anonymous interface parameter → error mentioning anonymous
	t.Run("anonymous interface", func(t *testing.T) {
		write("iface.go", `package iface
func useAnon(i interface{ M() }) {}
`)
		_, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "useAnon"})
		if err == nil {
			t.Fatal("expected error for anonymous interface, got nil")
		}
		if !strings.Contains(err.Error(), "anonymous") {
			t.Fatalf("error should mention anonymous, got: %v", err)
		}
	})

	// Test 9: Cycle detection
	t.Run("cycle detection", func(t *testing.T) {
		write("iface.go", `package iface
type Iface interface { M() }
type Node struct { Next Iface }
func (Node) M() {}
func useNode(n Iface) {}
`)
		_, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "useNode"})
		if err == nil {
			t.Fatal("expected error for cycle, got nil")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("error should mention cycle, got: %v", err)
		}
	})

	// Test 10: Control - non-flattenable type skipped, flattenable one chosen
	t.Run("non-flattenable skipped, flattenable chosen", func(t *testing.T) {
		write("iface.go", `package iface
type Reader interface { Read([]byte) (int, error) }
type BadReader struct { Ch chan int }
func (BadReader) Read(b []byte) (int, error) { return 0, nil }
type GoodReader struct { Data []byte }
func (GoodReader) Read(b []byte) (int, error) { return 0, nil }
func useReader(r Reader) {}
`)
		target, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "useReader"})
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		// Should choose GoodReader, not error
		if !strings.Contains(target.ArgumentTemplates[0], "GoodReader{") {
			t.Fatalf("should choose GoodReader, got: %q", target.ArgumentTemplates[0])
		}
		got := target.Implementations
		if !reflect.DeepEqual(got, []string{"Reader=GoodReader"}) {
			t.Fatalf("got Implementations=%q, want [\"Reader=GoodReader\"]", got)
		}
	})

	// Test 11: Interface satisfying itself is rejected, concrete type chosen
	// Regression test for: types.Implements(Reader, Reader) is true, so a naive
	// scan offers the interface as an implementation of itself. This leads to
	// infinite recursion unless the candidate's underlying type is checked for
	// being an interface.
	t.Run("interface satisfies itself but is rejected", func(t *testing.T) {
		write("iface.go", `package iface
type Reader interface { Read([]byte) (int, error) }
type ReadCloser interface { Read([]byte) (int, error); Close() error }
type Buffer struct { Data []byte; Pos int }
func (b *Buffer) Read(p []byte) (int, error) { return 0, nil }
func (b *Buffer) Close() error { return nil }
func useReader(r Reader) {}
`)
		target, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "useReader"})
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		// Must choose the concrete type Buffer, not error on ambiguity with ReadCloser interface
		if !strings.Contains(target.ArgumentTemplates[0], "Buffer{") {
			t.Fatalf("should choose Buffer, got: %q", target.ArgumentTemplates[0])
		}
		got := target.Implementations
		if !reflect.DeepEqual(got, []string{"Reader=Buffer"}) {
			t.Fatalf("got Implementations=%q, want [\"Reader=Buffer\"]", got)
		}
	})
}
