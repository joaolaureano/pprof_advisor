package benchgen

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactsCollisionAndInstall(t *testing.T) {
	dir := t.TempDir()
	target := Target{Dir: dir, Function: "parse"}
	o := Options{Out: dir, Write: true}
	result, err := writeArtifacts(o, target, []Seed{{Hash: "a", Origins: []string{"seed"}}}, []byte("code"))
	if err != nil {
		t.Fatal(err)
	}
	if result.InstalledPath != result.CodePath || !result.Generated || result.Validated {
		t.Fatalf("wrong report: %+v", result)
	}
	if _, err := writeArtifacts(o, target, nil, []byte("replacement")); err == nil {
		t.Fatal("overwrote existing file")
	}
	content, err := os.ReadFile(result.CodePath)
	if err != nil || string(content) != "code" {
		t.Fatalf("changed existing artifact: %q %v", content, err)
	}
}

func TestArtifactsPreflightAndRollback(t *testing.T) {
	for _, preexisting := range []bool{true, false} {
		t.Run(map[bool]string{true: "preflight", false: "rollback"}[preexisting], func(t *testing.T) {
			root := t.TempDir()
			out := filepath.Join(root, "out")
			targetDir := filepath.Join(root, "missing")
			if preexisting {
				targetDir = root
				if err := os.WriteFile(filepath.Join(root, "profadvisor_parse_test.go"), []byte("existing"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := writeArtifacts(Options{Out: out, Write: true}, Target{Dir: targetDir, Function: "parse"}, nil, []byte("code"))
			if err == nil {
				t.Fatal("expected failed installation")
			}
			for _, name := range []string{"profadvisor_parse_test.go", "profadvisor_parse.manifest.json"} {
				if _, err := os.Stat(filepath.Join(out, name)); !os.IsNotExist(err) {
					t.Fatalf("left artifact %s: %v", name, err)
				}
			}
		})
	}
}

func TestArtifactsManifestStable(t *testing.T) {
	target := Target{Dir: "/stable", Function: "parse"}
	seeds := []Seed{{Hash: "b", Origins: []string{"z", "a"}}, {Hash: "a", Origins: []string{"x"}}}
	first, err := writeArtifacts(Options{Out: t.TempDir()}, target, seeds, []byte("code"))
	if err != nil {
		t.Fatal(err)
	}
	seeds[0], seeds[1] = seeds[1], seeds[0]
	second, err := writeArtifacts(Options{Out: t.TempDir()}, target, seeds, []byte("code"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := os.ReadFile(first.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(second.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("unstable manifest: %s\n%s", a, b)
	}
}
