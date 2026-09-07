package render

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaolaureano/profadvisor/internal/capture"
	"github.com/joaolaureano/profadvisor/internal/escape"
	"github.com/joaolaureano/profadvisor/internal/extract"
	"github.com/joaolaureano/profadvisor/internal/fixture"
	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/pipeline"
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

// TestTextDiagnosis tests rendering of a Diagnosis.
func TestTextDiagnosis(t *testing.T) {
	r := &schema.Diagnosis{
		SchemaVersion: schema.Version,
		Provider:      "openai",
		Model:         "gpt-4",
		Measurement:   measurement.Config{Profile: "cpu", Unit: "ns/op", SampleUnit: "ns"},
		Target:        "example.com/pkg.someFunction",
		Cause:         "The function allocates frequently in a loop",
		Change:        "Use a sync.Pool to reuse the allocated buffer",
		Confidence:    "high",
		Risks:         []string{"The pool may not be thread-safe in all contexts"},
		Diff: `--- a/pkg/file.go
+++ b/pkg/file.go
@@ -10,7 +10,7 @@ func someFunction(x int) {
     for i := 0; i < x; i++ {
-        buf := make([]byte, 1024)
+        buf := pool.Get().([]byte)
         process(buf)
+        pool.Put(buf)
     }
 }
`,
	}
	got, err := Text(r)
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "diagnosis.txt", got)
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

// TestTextPipeline tests rendering of a pipeline.Result.
// Pipeline.Result is tested indirectly via cmd tests since we cannot import
// pipeline here without creating an import cycle (pipeline imports analyze which imports render).
// The reflection-based handling is covered by TestTextUnknownType and manual testing.

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

// TestTextPipelineRendersEveryStage covers the aggregate. It was the one
// renderer with no test while pipeline.Result was reached through reflection,
// because a test that named the type recreated the import cycle the reflection
// existed to dodge — the most fragile path was the uncovered one.
func TestTextPipelineRendersEveryStage(t *testing.T) {
	cfg, err := measurement.Resolve(measurement.CPU, "ns/op")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Text(&pipeline.Result{
		SchemaVersion: schema.Version,
		Measurement:   cfg,
		Baseline: &capture.Result{
			Dir: "out/20260907T120000.000Z", Measurement: cfg,
			ProfilePath: "out/20260907T120000.000Z/cpu.prof",
			BenchPath:   "out/20260907T120000.000Z/bench.txt",
			Command:     []string{"go", "test", "./...", "-bench", "."},
			Duration:    12 * time.Second,
		},
		Diagnosis: &schema.Diagnosis{
			SchemaVersion: schema.Version, Provider: "acme", Model: "m-1",
			Measurement: cfg, Target: "pkg.Fn", Cause: "linear scan",
			Change: "index it", Diff: "--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-a\n+b\n",
			Confidence: "medium", Risks: []string{"ordering"},
		},
		Applied: &schema.ApplyResult{
			SchemaVersion: schema.Version, Branch: "profadvisor/suggestion-1",
			BaseRef: "main", Commit: "abc1234", FilesChanged: []string{"x.go"},
		},
		After: &capture.Result{
			Dir: "out/20260907T120500.000Z", Measurement: cfg,
			ProfilePath: "out/20260907T120500.000Z/cpu.prof",
			BenchPath:   "out/20260907T120500.000Z/bench.txt",
			Command:     []string{"go", "test", "./...", "-bench", "."},
			Duration:    11 * time.Second,
		},
		Duration: 40 * time.Second,
	})
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	// Hotspots is nil here on purpose: a stage that did not run must be named,
	// not silently skipped, or the reader cannot tell it apart from a stage
	// that ran and found nothing.
	if !strings.Contains(got, "did not run") {
		t.Error("a nil stage was rendered as if it had run")
	}
	// The run stops before judging; the output has to say what judges.
	if !strings.Contains(got, "profadvisor verify --baseline ") {
		t.Error("the record does not name the verify invocation that follows it")
	}
	compareGolden(t, "pipeline.txt", got)
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
