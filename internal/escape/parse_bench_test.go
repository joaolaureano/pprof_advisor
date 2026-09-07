package escape

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/joaolaureano/profadvisor/internal/fixture"
)

// BenchmarkParse measures the performance of parsing compiler escape-analysis
// diagnostics at three input scales.
//
// A real project typically produces 5000-10000 lines of escape output;
// benchmarking only 86 lines (the testdata fixture) would measure call overhead
// rather than the algorithm's scaling. So we scale the input internally with
// sub-benchmarks at 1x, 10x, and 100x the fixture size, named by approximate
// line count.
//
// When the fixture is repeated, file names are rewritten per repetition
// (append.go → append_1.go, append_2.go, etc.) because Parse keys pending
// blocks by "file:line:col". Repeating without rewriting would collide
// positions across copies and change the measured behavior.
func BenchmarkParse(b *testing.B) {
	// Load the fixture text once.
	fixtureText, err := loadFixtureContent()
	if err != nil {
		b.Fatalf("load fixture: %v", err)
	}
	// The package header is dropped here and re-emitted once per repetition
	// below, with a distinct import path. Real output has one header per
	// package block, and keeping the original would either repeat the same
	// path (which no compiler emits) or, if stripped entirely, leave every
	// finding with an empty Package and never exercise the header branch.
	var baseText []byte
	for _, line := range strings.Split(strings.TrimSpace(fixtureText), "\n") {
		if !strings.HasPrefix(line, "# ") {
			baseText = append(baseText, []byte(line+"\n")...)
		}
	}

	cases := []struct {
		name    string
		repeats int
	}{
		{"lines=86", 1},
		{"lines=860", 10},
		{"lines=8600", 100},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			// Build scaled input by repeating and rewriting file names.
			scaled := rewriteFilesInFixture(baseText, tc.repeats)

			// Validate once that the benchmark measures the recognized path.
			result, err := Parse(scaled, "go1.26.1")
			if err != nil {
				b.Fatalf("parse error: %v", err)
			}
			if len(result.Unrecognized) > 0 {
				b.Fatalf("fixture has %d unrecognized lines; benchmark would measure error path", len(result.Unrecognized))
			}
			if len(result.Findings) == 0 {
				b.Fatal("fixture produced no findings")
			}
			b.ReportAllocs()

			// Benchmark the Parse function.
			b.ResetTimer()
			for b.Loop() {
				_, _ = Parse(scaled, "go1.26.1")
			}
		})
	}
}

// loadFixtureContent reads the raw escape fixture file.
func loadFixtureContent() (string, error) {
	path, err := fixture.Path("escape/raw-go1.26.1.txt")
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// rewriteFilesInFixture repeats the escape output text 'repeats' times as one
// package block per repetition, rewriting all .go file names in each so
// positions do not collide: Parse keys pending explanation blocks by
// "file:line:col", and a heading from one copy binding to a summary in another
// would change the behaviour being measured.
// The corpus files are: append.go, closure.go, heap.go, iface.go, params.go, stack.go.
func rewriteFilesInFixture(base []byte, repeats int) []byte {
	corpusFiles := []string{"append.go", "closure.go", "heap.go", "iface.go", "params.go", "stack.go"}

	var result bytes.Buffer
	for rep := 0; rep < repeats; rep++ {
		fmt.Fprintf(&result, "# example.com/escapecorpus%d\n", rep+1)
		text := string(base)
		// Rewrite each corpus file name to avoid position collisions.
		for _, file := range corpusFiles {
			nameWithoutExt := strings.TrimSuffix(file, ".go")
			// Within this repetition, use the suffix _rep (e.g., append_1.go, append_2.go).
			newName := fmt.Sprintf("%s_%d.go", nameWithoutExt, rep+1)
			text = strings.ReplaceAll(text, "./"+file, "./"+newName)
		}
		result.WriteString(text)
	}
	return result.Bytes()
}
