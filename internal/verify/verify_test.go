package verify

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaolaureano/profadvisor/internal/schema"
)

func TestFasterIsImproved(t *testing.T) {
	result := compareFixtures(t, "bench-baseline.txt", "bench-after-faster.txt")
	if result.Verdict != schema.VerdictImproved {
		t.Fatalf("Verdict = %q, want %q", result.Verdict, schema.VerdictImproved)
	}
	for _, comparison := range result.Comparisons {
		if !comparison.Significant {
			t.Errorf("%s was not significant", comparison.Name)
		}
		if comparison.DeltaPct >= -20 || comparison.DeltaPct <= -30 {
			t.Errorf("%s DeltaPct = %.2f, want near -25", comparison.Name, comparison.DeltaPct)
		}
	}
}

func TestSlowerIsRegressed(t *testing.T) {
	result := compareFixtures(t, "bench-baseline.txt", "bench-after-slower.txt")
	if result.Verdict != schema.VerdictRegressed {
		t.Fatalf("Verdict = %q, want %q", result.Verdict, schema.VerdictRegressed)
	}
	for _, comparison := range result.Comparisons {
		if comparison.DeltaPct <= 20 || comparison.DeltaPct >= 30 {
			t.Errorf("%s DeltaPct = %.2f, want near +25", comparison.Name, comparison.DeltaPct)
		}
	}
}

func TestSameIsNoChange(t *testing.T) {
	result := compareFixtures(t, "bench-baseline.txt", "bench-after-same.txt")
	if result.Verdict != schema.VerdictNoChange {
		t.Fatalf("Verdict = %q, want %q", result.Verdict, schema.VerdictNoChange)
	}
	for _, comparison := range result.Comparisons {
		if comparison.Significant {
			t.Errorf("%s was unexpectedly significant", comparison.Name)
		}
	}
}

func TestRegressionWinsOverImprovement(t *testing.T) {
	baseline := "BenchmarkFast-8 1 100 ns/op\nBenchmarkSlow-8 1 100 ns/op\n"
	after := "BenchmarkFast-8 1 50 ns/op\nBenchmarkSlow-8 1 200 ns/op\n"
	for i := 0; i < 7; i++ {
		baseline += "BenchmarkFast-8 1 100 ns/op\nBenchmarkSlow-8 1 100 ns/op\n"
		after += "BenchmarkFast-8 1 50 ns/op\nBenchmarkSlow-8 1 200 ns/op\n"
	}
	result, err := FromReaders(strings.NewReader(baseline), "baseline", strings.NewReader(after), "after", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != schema.VerdictRegressed {
		t.Fatalf("Verdict = %q, want %q", result.Verdict, schema.VerdictRegressed)
	}
}

func TestSingleSampleDoesNotClaimSignificance(t *testing.T) {
	result := compareFixtures(t, "bench-single-sample.txt", "bench-single-sample.txt")
	for _, comparison := range result.Comparisons {
		if comparison.Significant {
			t.Errorf("%s was unexpectedly significant", comparison.Name)
		}
	}
	if len(result.Warnings) == 0 {
		t.Error("Warnings is empty")
	}
}

func TestMissingBenchmarkWarns(t *testing.T) {
	baseline := "BenchmarkPresent-8 1 100 ns/op\nBenchmarkMissing-8 1 100 ns/op\n"
	after := "BenchmarkPresent-8 1 100 ns/op\n"
	result, err := FromReaders(strings.NewReader(baseline), "baseline", strings.NewReader(after), "after", Options{})
	if err != nil {
		t.Fatal(err)
	}
	// benchfmt strips the leading "Benchmark" from the names it reports, so
	// a line written as BenchmarkPresent-8 comes back as Present-8. Do not
	// "fix" this back — the implementation is reporting what benchfmt gives it.
	if len(result.Comparisons) != 1 || result.Comparisons[0].Name != "Present-8" {
		t.Fatalf("Comparisons = %#v, want only Present-8", result.Comparisons)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "Missing-8") {
		t.Errorf("Warnings = %#v, want a warning naming Missing-8", result.Warnings)
	}
}

func TestEmptyInputIsError(t *testing.T) {
	_, err := FromReaders(bytes.NewReader(nil), "empty-baseline", strings.NewReader("BenchmarkOne-8 1 1 ns/op\n"), "after", Options{})
	if err == nil || !strings.Contains(err.Error(), "empty-baseline") {
		t.Fatalf("error = %v, want name of empty input", err)
	}
}

func TestDeterministicOrder(t *testing.T) {
	input := "BenchmarkZulu-8 1 1 ns/op\nBenchmarkAlpha-8 1 1 ns/op\nBenchmarkMiddle-8 1 1 ns/op\n"
	result, err := FromReaders(strings.NewReader(input), "baseline", strings.NewReader(input), "after", Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(result.Comparisons); i++ {
		if result.Comparisons[i-1].Name > result.Comparisons[i].Name {
			t.Fatalf("Comparisons are not sorted: %#v", result.Comparisons)
		}
	}
}

func compareFixtures(t *testing.T, baseline, after string) *schema.VerifyResult {
	t.Helper()
	result, err := FromFiles(filepath.Join("..", "..", "testdata", baseline), filepath.Join("..", "..", "testdata", after), Options{})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFixturesExist(t *testing.T) {
	for _, name := range []string{"bench-baseline.txt", "bench-after-same.txt", "bench-after-faster.txt", "bench-after-slower.txt", "bench-single-sample.txt"} {
		if _, err := os.Stat(filepath.Join("..", "..", "testdata", name)); err != nil {
			t.Fatal(err)
		}
	}
}
