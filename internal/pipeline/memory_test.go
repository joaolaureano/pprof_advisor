package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaolaureano/profadvisor/internal/capture"
	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

const memorySource = `package slowpkg

//go:noinline
func clone(data []byte) []byte {
	copyOfData := make([]byte, len(data))
	copy(copyOfData, data)
	return copyOfData
}

func Sum(data []byte) int {
	for i := 0; i < 16; i++ { data = clone(data) }
	total := 0
	for _, value := range data {
		total += int(value)
	}
	return total
}
`

const memoryBenchmark = `package slowpkg
import "testing"
var result int
func BenchmarkSum(b *testing.B) {
	data := make([]byte, 4096)
	for i := range data { data[i] = byte(i) }
	for b.Loop() { result = Sum(data) }
}
`

func memoryRepo(t *testing.T, after string) (string, string) {
	t.Helper()
	dir := seedRepo(t)
	for name, body := range map[string]string{"slow.go": memorySource, "slow_test.go": memoryBenchmark} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-m", "allocation fixture")
	if err := os.WriteFile(filepath.Join(dir, "slow.go"), []byte(after), 0o644); err != nil {
		t.Fatal(err)
	}
	diff := gitOut(t, dir, "diff") + "\n"
	gitOut(t, dir, "checkout", "--", "slow.go")
	return dir, diff
}

func TestMemoryPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("runs real profiling and benchmark captures")
	}
	optimized := strings.Replace(memorySource, "\tfor i := 0; i < 16; i++ { data = clone(data) }\n", "", 1)
	// Large, deliberate effects keep real benchmark assertions away from noise.
	slow := strings.Replace(optimized, "\tfor _, value := range data {\n\t\ttotal += int(value)\n\t}", "\tfor repeat := 0; repeat < 1000; repeat++ {\n\t\ttotal = 0\n\t\tfor _, value := range data { total += int(value) }\n\t}", 1)
	for _, tc := range []struct {
		name, source string
		wantAccepted bool
		wantError    bool
	}{
		{"improvement", optimized, true, false},
		{"cpu_regression", slow, false, false},
		{"after_compile_failure", strings.Replace(optimized, "return total", "return undefined", 1), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, diff := memoryRepo(t, tc.source)
			base := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
			res, err := Run(context.Background(), stubClient{diff: diff}, Options{
				Profile: measurement.Memory,
				Capture: capture.Options{Dir: dir, Pkg: ".", Bench: "^BenchmarkSum$", Count: 10, Benchtime: "100ms", OutDir: t.TempDir(), Timeout: time.Minute},
			})
			if !tc.wantError {
				if err != nil {
					t.Fatal(err)
				}
				wantVerdict := schema.VerdictImproved
				if !tc.wantAccepted {
					wantVerdict = schema.VerdictRegressed
				}
				if res.Accepted != tc.wantAccepted || res.Verification.Verdict != wantVerdict {
					t.Fatalf("verification: %+v", res.Verification)
				}
				if res.Measurement.Profile != measurement.Memory || res.Diagnosis.Measurement != res.Measurement || res.Hotspots.Profile.Measurement != res.Measurement || res.Verification.Measurement != res.Measurement {
					t.Fatal("measurement lost between stages")
				}
				var objective, guard bool
				for _, c := range res.Verification.Comparisons {
					objective = objective || c.Unit == "B/op" && c.Role == measurement.Objective && c.Verdict == schema.VerdictImproved
					guard = guard || c.Unit == "ns/op" && c.Role == measurement.Guard && ((c.Verdict != schema.VerdictRegressed) == tc.wantAccepted)
				}
				if !objective || !guard {
					t.Fatalf("missing objective or CPU protection: %+v", res.Verification)
				}
			} else if err == nil || res.Accepted || res.Applied == nil {
				t.Fatalf("expected failure after apply: %+v, %v", res, err)
			}
			if got := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); got != base {
				t.Fatalf("left on %q instead of %q", got, base)
			}
			if res.Applied == nil || gitOut(t, dir, "branch", "--list", res.Applied.Branch) == "" {
				t.Fatal("suggestion branch lost")
			}
		})
	}
}

func TestInvalidRunConfigurationDoesNotCapture(t *testing.T) {
	for _, opts := range []Options{
		{Profile: measurement.Memory, Unit: "ns/op"},
		{Profile: measurement.Memory, Capture: capture.Options{Profile: measurement.CPU}},
		{Capture: capture.Options{Count: 1}},
	} {
		res, err := Run(context.Background(), nil, opts)
		if err == nil || res.Baseline != nil {
			t.Fatalf("invalid run started capture: %+v %v", res, err)
		}
	}
}
