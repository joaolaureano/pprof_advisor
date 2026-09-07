package benchgen

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	if err != nil {
		t.Fatalf("failed to read artifact: %v", err)
	}
	// The record must have the build constraint prefix.
	expected := "//go:build ignore\n\ncode"
	if string(content) != expected {
		t.Fatalf("changed existing artifact: %q (expected %q)", content, expected)
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

func TestArtifactsOutRecordIsInert(t *testing.T) {
	root := t.TempDir()
	outDir := filepath.Join(root, "out")
	targetDir := filepath.Join(root, "target")
	if err := os.Mkdir(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	target := Target{Dir: targetDir, Function: "parse"}
	code := []byte("package foo\nfunc Test() {}")
	o := Options{Out: outDir, Write: true}
	result, err := writeArtifacts(o, target, []Seed{{Hash: "a", Origins: []string{"seed"}}}, code)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Read the record written to --out.
	record, err := os.ReadFile(result.CodePath)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}

	// The record must begin with the build constraint and a blank line.
	const prefix = "//go:build ignore\n\n"
	if !bytes.HasPrefix(record, []byte(prefix)) {
		t.Fatalf("record does not start with build constraint.\nGot prefix: %q\nWant prefix: %q", record[:len(prefix)], prefix)
	}

	// The record without the prefix must be identical to the generated code.
	recordContent := record[len(prefix):]
	if !bytes.Equal(recordContent, code) {
		t.Fatalf("record content differs from generated code.\nGot: %q\nWant: %q", recordContent, code)
	}

	// Read the installed file written to the target package.
	installed, err := os.ReadFile(result.InstalledPath)
	if err != nil {
		t.Fatalf("read installed: %v", err)
	}

	// The installed file must NOT contain the build constraint and must be byte-identical to the generated code.
	if !bytes.Equal(installed, code) {
		t.Fatalf("installed file differs from generated code.\nGot: %q\nWant: %q", installed, code)
	}
	if bytes.HasPrefix(installed, []byte("//go:build ignore")) {
		t.Fatal("installed file must not contain build constraint")
	}

	// Verify that CodeHash hashes the generated code, not the inert record.
	hash := sha256.Sum256(code)
	expectedHash := hex.EncodeToString(hash[:])
	if result.Manifest.CodeHash != expectedHash {
		t.Fatalf("CodeHash mismatch.\nGot: %s\nWant: %s", result.Manifest.CodeHash, expectedHash)
	}
}

func TestArtifactsOutRecordWithoutWrite(t *testing.T) {
	dir := t.TempDir()
	target := Target{Dir: "/unrelated", Function: "parse"}
	code := []byte("package foo\nfunc Test() {}")
	o := Options{Out: dir, Write: false}
	result, err := writeArtifacts(o, target, []Seed{{Hash: "a", Origins: []string{"seed"}}}, code)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Read the record written to --out.
	record, err := os.ReadFile(result.CodePath)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}

	// Even without --write, the record must have the build constraint.
	const prefix = "//go:build ignore\n\n"
	if !bytes.HasPrefix(record, []byte(prefix)) {
		t.Fatalf("record without --write must still have build constraint.\nGot prefix: %q\nWant prefix: %q", record[:len(prefix)], prefix)
	}

	// Verify that CodeHash still hashes the generated code.
	hash := sha256.Sum256(code)
	expectedHash := hex.EncodeToString(hash[:])
	if result.Manifest.CodeHash != expectedHash {
		t.Fatalf("CodeHash mismatch.\nGot: %s\nWant: %s", result.Manifest.CodeHash, expectedHash)
	}

	// InstalledPath should be empty when Write is false.
	if result.InstalledPath != "" {
		t.Fatalf("InstalledPath should be empty when Write=false, got: %s", result.InstalledPath)
	}
}
