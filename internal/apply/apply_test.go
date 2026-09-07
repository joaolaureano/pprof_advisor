package apply

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaolaureano/profadvisor/internal/schema"
)

func TestAppliesOnNewBranch(t *testing.T) {
	dir := seedRepo(t)
	d := diagnosis(validDiff())
	res, err := Run(context.Background(), d, Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Branch != "profadvisor/suggestion-1" {
		t.Fatalf("Branch = %q", res.Branch)
	}
	if !strings.Contains(readFile(t, dir, "value.go"), "return 2") {
		t.Fatal("applied file does not contain change")
	}
	if got := gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); got != res.Branch {
		t.Fatalf("HEAD = %q, want %q", got, res.Branch)
	}
	gitRun(t, dir, "checkout", "master")
	if !strings.Contains(readFile(t, dir, "value.go"), "return 1") {
		t.Fatal("base branch was changed")
	}
}

func TestDirtyTreeIsRefused(t *testing.T) {
	dir := seedRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "value.go"), []byte("package seed\n\nfunc Value() int { return 3 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), diagnosis(validDiff()), Options{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("error = %v", err)
	}
	if got := gitOutput(t, dir, "branch", "--list", "profadvisor/*"); got != "" {
		t.Fatalf("unexpected branch: %q", got)
	}
}

func TestBadDiffIsRefusedAndLeavesNoBranch(t *testing.T) {
	dir := seedRepo(t)
	bad := strings.Replace(validDiff(), "return 1", "return 99", 1)
	_, err := Run(context.Background(), diagnosis(bad), Options{Dir: dir})
	if err == nil {
		t.Fatal("Run succeeded with an inapplicable diff")
	}
	if got := gitOutput(t, dir, "branch", "--list", "profadvisor/*"); got != "" {
		t.Fatalf("unexpected branch: %q", got)
	}
	if got := gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); got != "master" {
		t.Fatalf("HEAD = %q, want master", got)
	}
}

func TestBranchNumberIncrements(t *testing.T) {
	dir := seedRepo(t)
	first, err := Run(context.Background(), diagnosis(validDiff()), Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "checkout", "master")
	second, err := Run(context.Background(), diagnosis(validDiff()), Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if first.Branch != "profadvisor/suggestion-1" || second.Branch != "profadvisor/suggestion-2" {
		t.Fatalf("branches = %q, %q", first.Branch, second.Branch)
	}
}

func TestDryRunMutatesNothing(t *testing.T) {
	dir := seedRepo(t)
	head := gitOutput(t, dir, "rev-parse", "HEAD")
	res, err := Run(context.Background(), diagnosis(validDiff()), Options{Dir: dir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Branch != "" || res.Commit != "" || len(res.FilesChanged) == 0 {
		t.Fatalf("result = %+v", res)
	}
	if got := gitOutput(t, dir, "branch", "--list", "profadvisor/*"); got != "" {
		t.Fatalf("unexpected branch: %q", got)
	}
	if got := gitOutput(t, dir, "status", "--porcelain"); got != "" {
		t.Fatalf("working tree changed: %q", got)
	}
	if got := gitOutput(t, dir, "rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD = %q, want %q", got, head)
	}
}

func TestEmptyDiffIsError(t *testing.T) {
	_, err := Run(context.Background(), &schema.Diagnosis{}, Options{})
	if err == nil {
		t.Fatal("Run succeeded with empty diff")
	}
}

func TestApplyIgnoresUnrelatedUntrackedFiles(t *testing.T) {
	dir := seedRepo(t)
	// Create an untracked file in the output directory, simulating the
	// profadvisor-out/pkg.test binary that capture creates.
	outDir := filepath.Join(dir, "profadvisor-out", "x")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "pkg.test"), []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Applying should succeed despite the untracked file.
	d := diagnosis(validDiff())
	res, err := Run(context.Background(), d, Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Branch == "" {
		t.Fatalf("expected a branch, got empty string")
	}
	// The untracked file must NOT be in the commit.
	commit := gitOutput(t, dir, "show", "--name-only", "--pretty=format:", "HEAD")
	if strings.Contains(commit, "profadvisor-out") || strings.Contains(commit, "pkg.test") {
		t.Fatalf("untracked file unexpectedly in commit:\n%s", commit)
	}
	// The patched file must be in the commit.
	if !strings.Contains(commit, "value.go") {
		t.Fatalf("patched file not in commit:\n%s", commit)
	}
}

func TestApplyStillRefusesModifiedTrackedFiles(t *testing.T) {
	dir := seedRepo(t)
	// Modify a tracked file (value.go) without committing.
	if err := os.WriteFile(filepath.Join(dir, "value.go"), []byte("package seed\n\nfunc Value() int { return 3 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Attempting to apply should still fail due to the uncommitted change.
	d := diagnosis(validDiff())
	_, err := Run(context.Background(), d, Options{Dir: dir})
	if err == nil {
		t.Fatal("Run succeeded despite uncommitted tracked changes")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("error did not mention uncommitted changes: %v", err)
	}
	// No branch should have been created.
	if got := gitOutput(t, dir, "branch", "--list", "profadvisor/*"); got != "" {
		t.Fatalf("unexpected branch: %q", got)
	}
}

func seedRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "master")
	gitRun(t, dir, "config", "user.email", "apply-test@example.com")
	gitRun(t, dir, "config", "user.name", "Apply Test")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "value.go"), []byte("package seed\n\nfunc Value() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "value.go")
	gitRun(t, dir, "commit", "-m", "seed")
	return dir
}

func diagnosis(diff string) *schema.Diagnosis {
	return &schema.Diagnosis{Diff: diff, Change: "make Value faster", Cause: "test change", Target: "seed.Value", Confidence: "high"}
}

func validDiff() string {
	return "diff --git a/value.go b/value.go\nindex 2f154fa..9bcbabe 100644\n--- a/value.go\n+++ b/value.go\n@@ -1,3 +1,3 @@\n package seed\n \n-func Value() int { return 1 }\n+func Value() int { return 2 }\n"
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
