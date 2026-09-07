package benchgen

import (
	"context"
	"os"
	"path/filepath"
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
 func alias(a Alias) {}
 func named(n Named) {}
 func generic[T any](s string) {}
 func variadic(s ...string) {}
 func zero() {}
 type T struct{}
 func (T) method(s string) {}
 `)
	for _, name := range []string{"hidden", "bytes", "alias", "number", "flag", "ratio", "runeArg", "named", "generic", "variadic", "zero", "method", "missing"} {
		t.Run(name, func(t *testing.T) {
			target, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: name})
			wantOK := name == "hidden" || name == "bytes" || name == "alias" || name == "number" || name == "flag" || name == "ratio" || name == "runeArg"
			if (err == nil) != wantOK {
				t.Fatalf("target %+v, error %v", target, err)
			}
			if wantOK && target.FuzzName != "FuzzProfadvisor_"+name {
				t.Fatal("wrong name")
			}
		})
	}
	write("conflict_test.go", "package target\nimport \"testing\"\nfunc FuzzProfadvisor_hidden(f *testing.F) {}\n")
	if _, err := resolveTarget(context.Background(), Options{Dir: dir, Package: ".", Function: "hidden"}); err == nil {
		t.Fatal("accepted existing symbol")
	}
}
