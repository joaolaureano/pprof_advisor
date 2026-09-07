package render

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaolaureano/profadvisor/internal/benchgen"
	"github.com/joaolaureano/profadvisor/internal/capture"
	"github.com/joaolaureano/profadvisor/internal/escape"
	"github.com/joaolaureano/profadvisor/internal/extract"
	"github.com/joaolaureano/profadvisor/internal/fixture"
	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

var update = flag.Bool("update", false, "rewrite the text golden files")

func compareGolden(t *testing.T, name, got string) {
	t.Helper()
	dir, err := fixture.Dir()
	if err != nil {
		t.Fatal(err)
	}
	// The rendered output names the source file by absolute path, which is
	// correct at runtime and wrong in a golden file: it would pin these tests
	// to one checkout at one path, which is the very thing internal/fixture
	// re-anchors profiles to avoid. The checkout root is replaced by a
	// placeholder so the goldens travel with the repository.
	got = strings.ReplaceAll(got, dir, "<testdata>")
	path := filepath.Join(dir, "render", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (run with -update to create it)", name, err)
	}
	if got != string(want) {
		t.Errorf("%s differs from the golden file; rerun with -update and review the diff", name)
	}
}

// TestTextCapture tests rendering of a capture.Result.
func TestTextCapture(t *testing.T) {
	r := &capture.Result{
		Dir:         "/tmp/profadvisor-out/20240101T120000.000Z",
		Measurement: measurement.Config{Profile: "cpu", Unit: "ns/op", SampleUnit: "ns", SampleType: "cpu"},
		ProfilePath: "/tmp/profadvisor-out/20240101T120000.000Z/cpu.prof",
		BenchPath:   "/tmp/profadvisor-out/20240101T120000.000Z/bench.txt",
		Command:     []string{"go", "test", "-bench", ".", "-benchmem", "-count", "10"},
		Duration:    time.Second * 30,
	}
	got, err := Text(r)
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "capture.txt", got)
}

// TestTextExtract tests rendering of an ExtractResult.
func TestTextExtract(t *testing.T) {
	p, err := fixture.Load("all.prof")
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	r, err := extract.FromProfile(p, "all.prof", extract.Options{TopN: 3})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	got, err := Text(r)
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "extract.txt", got)
}

// TestTextApply tests rendering of an ApplyResult.
func TestTextApply(t *testing.T) {
	r := &schema.ApplyResult{
		SchemaVersion: schema.Version,
		Branch:        "profadvisor-suggestion",
		BaseRef:       "main",
		Commit:        "abc123def456",
		FilesChanged:  []string{"pkg/file.go", "pkg/file_test.go"},
	}
	got, err := Text(r)
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "apply.txt", got)
}

// TestTextVerify tests rendering of a VerifyResult.
func TestTextVerify(t *testing.T) {
	r := &schema.VerifyResult{
		SchemaVersion: schema.Version,
		Measurement:   measurement.Config{Profile: "cpu", Unit: "ns/op", SampleUnit: "ns"},
		Verdict:       schema.VerdictImproved,
		Comparisons: []schema.BenchComparison{
			{
				Name:           "BenchmarkSomeFunction",
				Unit:           "ns/op",
				Role:           measurement.Objective,
				BaselineCenter: 100.5,
				AfterCenter:    85.3,
				BaselineN:      10,
				AfterN:         10,
				DeltaPct:       -15.15,
				PValue:         0.0005,
				Significant:    true,
				Verdict:        schema.VerdictImproved,
			},
			{
				Name:           "BenchmarkSomeFunction",
				Unit:           "B/op",
				Role:           "guard",
				BaselineCenter: 512.0,
				AfterCenter:    512.0,
				BaselineN:      10,
				AfterN:         10,
				DeltaPct:       0.0,
				PValue:         1.0,
				Significant:    false,
				Verdict:        schema.VerdictNoChange,
			},
		},
		Warnings: []string{"Sample size is small; consider increasing -count"},
	}
	got, err := Text(r)
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "verify.txt", got)
}

// TestTextEscape tests rendering of an EscapeReport.
func TestTextEscape(t *testing.T) {
	dir, err := fixture.Dir()
	if err != nil {
		t.Fatal(err)
	}
	rawPath := filepath.Join(dir, "escape", "raw-go1.26.1.txt")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatal(err)
	}

	// escape.Parse returns escape.Result, which we need to convert to schema.EscapeReport
	parseResult, err := escape.Parse(raw, "go1.26.1")
	if err != nil {
		t.Fatalf("escape.Parse: %v", err)
	}

	// Build a minimal EscapeReport for rendering test
	// In real usage, this would come from the escape command's full flow
	r := &schema.EscapeReport{
		SchemaVersion: schema.EscapeVersion,
		Toolchain: schema.Toolchain{
			Path:    "/usr/local/go/bin/go",
			Version: "go1.26.1",
			Raw:     "go version go1.26.1 linux/amd64",
			GOOS:    "linux",
			GOARCH:  "amd64",
		},
		Analysis: schema.EscapeAnalysis{
			Dir:               "/tmp/corpus",
			Patterns:          []string{"./..."},
			Command:           []string{"go", "build", "-m=2", "./..."},
			ParserProfile:     parseResult.Profile,
			TotalLines:        parseResult.Total,
			RecognizedLines:   parseResult.Recognized,
			IgnoredLines:      parseResult.Ignored,
			UnrecognizedLines: parseResult.Total - parseResult.Recognized - parseResult.Ignored,
			DurationNanos:     1000000000, // 1 second
		},
		Summary: schema.EscapeSummary{
			Packages: len(parseResult.Packages),
			Files:    len(parseResult.Files),
			Findings: len(parseResult.Findings),
			ByKind:   make(map[string]int),
		},
		Findings:     parseResult.Findings,
		Unrecognized: parseResult.Unrecognized,
		Warnings:     parseResult.Warnings,
	}

	// Count findings by kind
	for _, f := range parseResult.Findings {
		r.Summary.ByKind[string(f.Kind)]++
	}

	got, err := Text(r)
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "escape.txt", got)
}

// TestTextUnknownType should error with an unknown type.
func TestTextUnknownType(t *testing.T) {
	_, err := Text(42)
	if err == nil {
		t.Error("Text(42) should return an error")
	}
	if err.Error() != "Text: unsupported type int" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

// TestTextNilPointer should error on nil pointers.
func TestTextNilPointer(t *testing.T) {
	_, err := Text((*capture.Result)(nil))
	if err == nil {
		t.Error("Text(nil *capture.Result) should return an error")
	}
}

// TestTextRejectsAnUnknownDocument keeps Text honest: a document with no
// renderer must say so rather than fall back to something that looks fine.
func TestTextRejectsAnUnknownDocument(t *testing.T) {
	_, err := Text(42)
	if err == nil {
		t.Fatal("an unrenderable value was accepted")
	}
	if !strings.Contains(err.Error(), "int") {
		t.Errorf("error does not name the offending type: %v", err)
	}
}

// TestTextBenchgenWithImplementations asserts that Implementations line is rendered when present.
func TestTextBenchgenWithImplementations(t *testing.T) {
	r := &benchgen.Result{
		SchemaVersion: 4,
		Manifest: benchgen.Manifest{
			SchemaVersion:    4,
			GeneratorVersion: "4",
			Target: benchgen.Target{
				Package:         "mypackage",
				Function:        "myFunc",
				ArgumentTypes:   []string{"Reader", "Writer"},
				InputTypes:      []string{"[]byte"},
				Implementations: []string{"Reader=MyReader", "Writer=MyWriter"},
				GoVersion:       "go1.24",
				FuzzName:        "FuzzMyFunc",
				BenchmarkName:   "BenchmarkMyFunc",
			},
			CorpusHash: "abc123",
			CodeHash:   "def456",
			Seeds:      []benchgen.Seed{{Hash: "h1"}},
		},
		CodePath:     "/tmp/code.go",
		ManifestPath: "/tmp/manifest.json",
		Generated:    true,
		Validated:    false,
	}
	got, err := Text(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Implementations: Reader=MyReader, Writer=MyWriter") {
		t.Fatalf("got does not contain Implementations line.\ngot:\n%s", got)
	}
}

// TestTextBenchgenWithoutImplementations asserts that Implementations line is NOT rendered when absent.
func TestTextBenchgenWithoutImplementations(t *testing.T) {
	r := &benchgen.Result{
		SchemaVersion: 4,
		Manifest: benchgen.Manifest{
			SchemaVersion:    4,
			GeneratorVersion: "4",
			Target: benchgen.Target{
				Package:         "mypackage",
				Function:        "myFunc",
				ArgumentTypes:   []string{"string"},
				InputTypes:      []string{"string"},
				Implementations: []string{},
				GoVersion:       "go1.24",
				FuzzName:        "FuzzMyFunc",
				BenchmarkName:   "BenchmarkMyFunc",
			},
			CorpusHash: "abc123",
			CodeHash:   "def456",
			Seeds:      []benchgen.Seed{{Hash: "h1"}},
		},
		CodePath:     "/tmp/code.go",
		ManifestPath: "/tmp/manifest.json",
		Generated:    true,
		Validated:    false,
	}
	got, err := Text(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "Implementations") {
		t.Fatalf("got contains Implementations line when it should not.\ngot:\n%s", got)
	}
}
