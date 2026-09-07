package capture

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/joaolaureano/profadvisor/internal/fixture"
	"github.com/joaolaureano/profadvisor/internal/measurement"
)

// TestRunAgainstFixtureModule is an end-to-end check: a real `go test -bench`
// against the module in testdata/fixture, which ships with this repository so
// the test needs nothing checked out beside it.
func TestRunAgainstFixtureModule(t *testing.T) {
	if testing.Short() {
		t.Skip("runs a benchmark")
	}
	target, err := fixture.Path("fixture")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), Options{
		Pkg:       "./matcher/",
		Bench:     "^BenchmarkParam$",
		Dir:       target,
		Count:     3,
		Benchtime: "200x",
		OutDir:    t.TempDir(),
		Timeout:   3 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, p := range []string{res.ProfilePath, res.BenchPath} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if fi.Size() == 0 {
			t.Errorf("%s is empty", p)
		}
	}
	b, _ := os.ReadFile(res.BenchPath)
	t.Logf("cmd: %v", res.Command)
	t.Logf("bench output:\n%s", b)
}

func TestRunReportsNoMatchingBenchmarks(t *testing.T) {
	if testing.Short() {
		t.Skip("runs a benchmark")
	}
	target, err := fixture.Path("fixture")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), Options{
		Pkg:       "./matcher/",
		Bench:     "^NoSuchBenchmark$",
		Dir:       target,
		Count:     1,
		Benchtime: "1x",
		OutDir:    t.TempDir(),
		Timeout:   3 * time.Minute,
	})
	if err == nil {
		t.Fatal("Run succeeded with no matching benchmarks")
	}
	if !strings.Contains(err.Error(), "no benchmarks matched") {
		t.Fatalf("error = %v", err)
	}
	if res == nil {
		t.Fatal("Run returned nil result")
	}
	if _, statErr := os.Stat(res.BenchPath); statErr != nil {
		t.Fatalf("bench output was not preserved: %v", statErr)
	}
}

func TestProfileFlagsPerKind(t *testing.T) {
	tests := []struct {
		kind      measurement.Kind
		rate      int
		wantFile  string
		wantFlags []string
	}{
		{
			kind:      measurement.CPU,
			rate:      0,
			wantFile:  "cpu.prof",
			wantFlags: []string{"-cpuprofile", "/tmp/x.prof"},
		},
		{
			kind:      measurement.Memory,
			rate:      5,
			wantFile:  "mem.prof",
			wantFlags: []string{"-memprofile", "/tmp/x.prof"},
		},
		{
			kind:      measurement.Block,
			rate:      0,
			wantFile:  "block.prof",
			wantFlags: []string{"-blockprofile", "/tmp/x.prof", "-blockprofilerate", "1"},
		},
		{
			kind:      measurement.Block,
			rate:      7,
			wantFile:  "block.prof",
			wantFlags: []string{"-blockprofile", "/tmp/x.prof", "-blockprofilerate", "7"},
		},
		{
			kind:      measurement.Mutex,
			rate:      0,
			wantFile:  "mutex.prof",
			wantFlags: []string{"-mutexprofile", "/tmp/x.prof", "-mutexprofilefraction", "1"},
		},
		{
			kind:      measurement.Mutex,
			rate:      3,
			wantFile:  "mutex.prof",
			wantFlags: []string{"-mutexprofile", "/tmp/x.prof", "-mutexprofilefraction", "3"},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_rate%d", tt.kind, tt.rate), func(t *testing.T) {
			// Test profileFile
			file := profileFile(tt.kind)
			if file != tt.wantFile {
				t.Errorf("profileFile(%s) = %q, want %q", tt.kind, file, tt.wantFile)
			}

			// Test profileFlags
			cfg, err := measurement.Resolve(tt.kind, "")
			if err != nil {
				t.Fatalf("Resolve(%s, \"\"): %v", tt.kind, err)
			}
			flags := profileFlags(cfg, tt.rate, "/tmp/x.prof")
			if !slices.Equal(flags, tt.wantFlags) {
				t.Errorf("profileFlags(cfg, %d, \"/tmp/x.prof\") = %v, want %v", tt.rate, flags, tt.wantFlags)
			}
		})
	}
}
