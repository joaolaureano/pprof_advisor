package capture

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joaolaureano/profadvisor/internal/fixture"
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
