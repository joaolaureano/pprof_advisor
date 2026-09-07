package escape

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/joaolaureano/profadvisor/internal/fixture"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

func corpusDir(t *testing.T) string {
	t.Helper()
	dir, err := fixture.Dir()
	if err != nil {
		t.Fatalf("fixture.Dir: %v", err)
	}
	return filepath.Join(dir, "escape", "corpus")
}

// TestRunAgainstTheCorpus runs the real toolchain against testdata/escape/corpus.
// It asserts the basic structure of a successful report.
func TestRunAgainstTheCorpus(t *testing.T) {
	corpus := corpusDir(t)

	res, err := Run(context.Background(), Options{Dir: corpus})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res == nil {
		t.Fatal("res is nil")
	}

	if res.SchemaVersion != schema.EscapeVersion {
		t.Errorf("SchemaVersion: got %d, want %d", res.SchemaVersion, schema.EscapeVersion)
	}

	if res.Toolchain.Version == "" {
		t.Error("Toolchain.Version is empty")
	}

	if res.Toolchain.Path == "" {
		t.Error("Toolchain.Path is empty")
	}

	if !filepath.IsAbs(res.Toolchain.Path) {
		t.Errorf("Toolchain.Path is not absolute: %s", res.Toolchain.Path)
	}

	if len(res.Findings) == 0 {
		t.Fatal("no findings")
	}

	if res.Summary.Findings != len(res.Findings) {
		t.Errorf("Summary.Findings: got %d, want %d", res.Summary.Findings, len(res.Findings))
	}

	// Check that ByKind sums to Findings.
	byKindSum := 0
	for _, count := range res.Summary.ByKind {
		byKindSum += count
	}
	if byKindSum != res.Summary.Findings {
		t.Errorf("ByKind sum: got %d, want %d", byKindSum, res.Summary.Findings)
	}

	// Check that the command contains the expected gcflags.
	if len(res.Analysis.Command) == 0 {
		t.Fatal("Command is empty")
	}
	found := false
	for _, arg := range res.Analysis.Command {
		if arg == "-gcflags=-m=2" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Command does not contain -gcflags=-m=2: %v", res.Analysis.Command)
	}

	if !filepath.IsAbs(res.Analysis.Dir) {
		t.Errorf("Analysis.Dir is not absolute: %s", res.Analysis.Dir)
	}
}

// TestRunLeavesNothingUnrecognized checks that the corpus is fully recognized.
// This is a canary: when the compiler rewords a diagnostic, this test fires.
func TestRunLeavesNothingUnrecognized(t *testing.T) {
	corpus := corpusDir(t)

	res, err := Run(context.Background(), Options{Dir: corpus})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(res.Unrecognized) > 0 {
		t.Errorf("%d unrecognized lines:", len(res.Unrecognized))
		for _, u := range res.Unrecognized {
			t.Logf("  %s:%d:%d: %s", u.File, u.Line, u.Column, u.Text)
		}
		t.Fatal("unrecognized lines present; the parser may be behind the toolchain")
	}
}

// TestRunSortsFindingsDeterministically checks that findings are sorted
// by (Package, File, Line, Column, Kind) and that the order is stable.
func TestRunSortsFindingsDeterministically(t *testing.T) {
	corpus := corpusDir(t)

	// Run twice.
	res1, err := Run(context.Background(), Options{Dir: corpus})
	if err != nil {
		t.Fatalf("Run 1: %v", err)
	}

	res2, err := Run(context.Background(), Options{Dir: corpus})
	if err != nil {
		t.Fatalf("Run 2: %v", err)
	}

	// Check that res1 findings are sorted.
	for i := 1; i < len(res1.Findings); i++ {
		a := res1.Findings[i-1]
		b := res1.Findings[i]
		if a.Package > b.Package {
			t.Errorf("Package order: %s > %s", a.Package, b.Package)
		} else if a.Package == b.Package {
			if a.File > b.File {
				t.Errorf("File order: %s > %s", a.File, b.File)
			} else if a.File == b.File {
				if a.Line > b.Line {
					t.Errorf("Line order: %d > %d", a.Line, b.Line)
				} else if a.Line == b.Line {
					if a.Column > b.Column {
						t.Errorf("Column order: %d > %d", a.Column, b.Column)
					} else if a.Column == b.Column && string(a.Kind) > string(b.Kind) {
						t.Errorf("Kind order: %s > %s", a.Kind, b.Kind)
					}
				}
			}
		}
	}

	// Check that both runs produce the same findings order and structure.
	if len(res1.Findings) != len(res2.Findings) {
		t.Errorf("Finding count mismatch: %d vs %d", len(res1.Findings), len(res2.Findings))
		return
	}

	for i := range res1.Findings {
		f1 := res1.Findings[i]
		f2 := res2.Findings[i]
		// Compare the key fields that define a finding.
		if f1.Kind != f2.Kind || f1.Package != f2.Package || f1.File != f2.File ||
			f1.Line != f2.Line || f1.Column != f2.Column || f1.Subject != f2.Subject ||
			f1.Function != f2.Function || f1.Target != f2.Target || f1.Evidence != f2.Evidence {
			t.Errorf("Finding %d mismatch between runs", i)
		}
	}
}

// TestRunFailsOnATargetThatDoesNotCompile checks that a broken module is rejected.
func TestRunFailsOnATargetThatDoesNotCompile(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a broken Go module.
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.26\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc main() {\n\tinvalid syntax here\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main.go: %v", err)
	}

	res, err := Run(context.Background(), Options{Dir: tmpDir})

	if err == nil {
		t.Fatal("Run succeeded on broken module; expected an error")
	}

	if res != nil {
		t.Fatal("res is non-nil on error; must be (nil, err)")
	}
}

// TestRunRejectsAMissingDirectory checks that a non-existent directory is rejected.
func TestRunRejectsAMissingDirectory(t *testing.T) {
	res, err := Run(context.Background(), Options{Dir: "/nonexistent/path"})

	if err == nil {
		t.Fatal("Run succeeded on missing directory; expected an error")
	}

	if res != nil {
		t.Fatal("res is non-nil on error; must be (nil, err)")
	}
}
