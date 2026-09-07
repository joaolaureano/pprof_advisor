package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaolaureano/profadvisor/internal/apply"
	"github.com/joaolaureano/profadvisor/internal/capture"
	"github.com/joaolaureano/profadvisor/internal/extract"
	"github.com/joaolaureano/profadvisor/internal/llm"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

// The package under test: a deliberately slow lookup, and a benchmark that
// exercises it. `slow` scans a slice; the patch below replaces the scan with a
// map. The point is not the optimization — it is that the pipeline must arrive
// at MELHOROU by measuring, with no part of the run told what the answer is.
const targetGo = `package slowpkg

var table = func() []string {
	out := make([]string, 512)
	for i := range out {
		out[i] = string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + string(rune(i))
	}
	return out
}()

// Lookup reports whether key is in the table.
func Lookup(key string) bool {
	for _, v := range table {
		if v == key {
			return true
		}
	}
	return false
}
`

const targetBenchGo = `package slowpkg

import "testing"

func BenchmarkLookup(b *testing.B) {
	miss := "not-present-anywhere"
	b.ReportAllocs()
	for b.Loop() {
		Lookup(miss)
	}
}
`

// speedup replaces the linear scan with a map lookup. Generated with
// `git diff`, not written by hand: `git apply` is unforgiving about context,
// and a hand-counted hunk header is the most common way this test rots.
const speedupDiff = `diff --git a/slow.go b/slow.go
index a2617dc..8ef82a0 100644
--- a/slow.go
+++ b/slow.go
@@ -8,12 +8,16 @@ var table = func() []string {
 	return out
 }()
 
-// Lookup reports whether key is in the table.
-func Lookup(key string) bool {
+var index = func() map[string]struct{} {
+	m := make(map[string]struct{}, len(table))
 	for _, v := range table {
-		if v == key {
-			return true
-		}
+		m[v] = struct{}{}
 	}
-	return false
+	return m
+}()
+
+// Lookup reports whether key is in the table.
+func Lookup(key string) bool {
+	_, ok := index[key]
+	return ok
 }
`

// stubClient returns a fixed diagnosis, so the test measures the pipeline rather
// than the model. Everything downstream — apply, re-capture, verify — runs for
// real against a real git repository and a real Go toolchain.
type stubClient struct{ diff string }

func (s stubClient) Complete(context.Context, llm.Request) (*llm.Response, error) {
	body, _ := json.Marshal(map[string]any{
		"target": "slowpkg.Lookup", "cause": "linear scan over 512 entries",
		"change": "index the table in a map", "diff": s.diff,
		"confidence": "high", "risks": []string{},
	})
	return &llm.Response{Text: string(body), StopReason: "end_turn"}, nil
}

func seedRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module slowpkg\n\ngo 1.26\n")
	write("slow.go", targetGo)
	write("slow_test.go", targetBenchGo)
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-q", "-m", "seed"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// TestPipelineAcceptsARealImprovement is the end-to-end claim of this tool: a
// change is only accepted because it was measured, twice, on the same machine.
func TestPipelineAcceptsARealImprovement(t *testing.T) {
	if testing.Short() {
		t.Skip("runs two full benchmark suites")
	}
	dir := seedRepo(t)
	var progress strings.Builder

	res, err := Run(context.Background(), stubClient{diff: speedupDiff}, Options{
		Capture: capture.Options{
			Pkg: "./...", Bench: ".", Dir: dir, Count: 6,
			Benchtime: "500ms", OutDir: filepath.Join(t.TempDir(), "out"),
			Timeout: 4 * time.Minute,
		},
		Extract:  extract.Options{TopN: 5},
		Apply:    apply.Options{Dir: dir},
		Progress: &progress,
	})
	if err != nil {
		t.Fatalf("Run: %v\nprogress:\n%s", err, progress.String())
	}

	if !res.Accepted {
		t.Errorf("Accepted = false, verdict %q", res.Verification.Verdict)
	}
	if res.Verification.Verdict != schema.VerdictImproved {
		t.Errorf("verdict = %q, want %q", res.Verification.Verdict, schema.VerdictImproved)
	}
	if got := res.Applied.Branch; got != "profadvisor/suggestion-1" {
		t.Errorf("branch = %q", got)
	}

	// The repository must be back where it started, with the branch kept.
	head := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if head == res.Applied.Branch {
		t.Errorf("left the repo on the suggestion branch %q", head)
	}
	if branches := gitOut(t, dir, "branch", "--list", "profadvisor/*"); branches == "" {
		t.Error("suggestion branch was not kept for inspection")
	}

	// Every artifact is in the record, so a verdict can be investigated.
	if res.Baseline == nil || res.Hotspots == nil || res.Diagnosis == nil ||
		res.Applied == nil || res.After == nil || res.Verification == nil {
		t.Error("result is missing an intermediate artifact")
	}
	t.Logf("progress:\n%s", progress.String())
	for _, c := range res.Verification.Comparisons {
		t.Logf("%s: %.1f -> %.1f (%+.1f%%, p=%.4f) %s",
			c.Name, c.BaselineCenter, c.AfterCenter, c.DeltaPct, c.PValue, c.Verdict)
	}
}

// cosmeticDiff rewrites the range loop as an indexed one — the kind of change
// that reads like a micro-optimization. Measured, it is about 45% SLOWER: the
// indexed form reintroduces the bounds check that range elides.
const cosmeticDiff = `diff --git a/slow.go b/slow.go
index a2617dc..a3b21f4 100644
--- a/slow.go
+++ b/slow.go
@@ -10,8 +10,8 @@ var table = func() []string {
 
 // Lookup reports whether key is in the table.
 func Lookup(key string) bool {
-	for _, v := range table {
-		if v == key {
+	for i := 0; i < len(table); i++ {
+		if table[i] == key {
 			return true
 		}
 	}
`

// TestPipelineRejectsAPlausibleButWorseChange is the other half of the claim.
// A tool that only ever confirms is not measuring anything, so the rejection of
// a change that looks like an optimization has to be tested as deliberately as
// the acceptance. This one is not merely neutral — it regresses, and the
// pipeline is expected to say so rather than take the diff at face value.
func TestPipelineRejectsAPlausibleButWorseChange(t *testing.T) {
	if testing.Short() {
		t.Skip("runs two full benchmark suites")
	}
	dir := seedRepo(t)
	var progress strings.Builder

	res, err := Run(context.Background(), stubClient{diff: cosmeticDiff}, Options{
		Capture: capture.Options{
			Pkg: "./...", Bench: ".", Dir: dir, Count: 6,
			Benchtime: "500ms", OutDir: filepath.Join(t.TempDir(), "out"),
			Timeout: 4 * time.Minute,
		},
		Extract:  extract.Options{TopN: 5},
		Apply:    apply.Options{Dir: dir},
		Progress: &progress,
	})
	if err != nil {
		t.Fatalf("Run: %v\nprogress:\n%s", err, progress.String())
	}
	if res.Accepted {
		t.Error("a cosmetic rewrite was accepted as an improvement")
	}
	if res.Verification.Verdict == schema.VerdictImproved {
		t.Errorf("verdict = %q; the change is not faster", res.Verification.Verdict)
	}
	for _, c := range res.Verification.Comparisons {
		t.Logf("%s: %+.1f%% (p=%.4f) %s", c.Name, c.DeltaPct, c.PValue, c.Verdict)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}
