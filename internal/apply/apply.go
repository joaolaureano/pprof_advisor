// Package apply installs an analyzed change on an isolated Git branch.
//
// Applying an LLM-produced diff is the only pipeline step that mutates a
// repository. The checks in this package deliberately happen before creating a
// branch so a malformed suggestion cannot overwrite uncommitted work.
package apply

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/joaolaureano/profadvisor/internal/schema"
)

// Options controls where and how a diagnosis is applied.
type Options struct {
	// Dir is the target git repository root. Empty means the current directory.
	Dir string
	// BranchPrefix is the branch namespace. Empty means "profadvisor/suggestion".
	BranchPrefix string
	// Message overrides the commit message. Empty means a generated one.
	Message string
	// DryRun validates that the diff applies without creating a branch or
	// committing anything.
	DryRun bool
}

// Run applies d.Diff on a new branch and commits it.
func Run(ctx context.Context, d *schema.Diagnosis, opts Options) (*schema.ApplyResult, error) {
	if d == nil {
		return nil, fmt.Errorf("apply: diagnosis is required")
	}
	if d.Diff == "" {
		return nil, fmt.Errorf("apply: diagnosis diff is required")
	}

	// Check for tracked modifications only. The check exists so the suggestion
	// commit contains the patch and nothing else; precise staging (not a clean
	// working tree) is what actually guarantees that.
	status, stderr, err := git(ctx, opts.Dir, nil, "status", "--porcelain", "-uno")
	if err != nil {
		return nil, gitError("checking working tree", err, stderr)
	}
	if status != "" {
		return nil, fmt.Errorf("apply: working tree has uncommitted changes:\n%s", changedFiles(status))
	}

	baseRef, stderr, err := git(ctx, opts.Dir, nil, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, gitError("recording base branch", err, stderr)
	}
	baseRef = strings.TrimSpace(baseRef)
	_, stderr, err = git(ctx, opts.Dir, nil, "rev-parse", "HEAD")
	if err != nil {
		return nil, gitError("recording base commit", err, stderr)
	}

	files, stderr, err := numstat(ctx, opts.Dir, d.Diff)
	if err != nil {
		return nil, gitError("reading changed files", err, stderr)
	}
	_, stderr, err = git(ctx, opts.Dir, strings.NewReader(d.Diff), "apply", "--check", "-")
	if err != nil {
		// Git's diagnosis is actionable here: generated diffs most often fail
		// because their context no longer matches the checked-out source.
		return nil, gitError("validating diff", err, stderr)
	}

	res := &schema.ApplyResult{
		SchemaVersion: schema.Version,
		BaseRef:       baseRef,
		FilesChanged:  files,
	}
	if opts.DryRun {
		return res, nil
	}

	prefix := opts.BranchPrefix
	if prefix == "" {
		prefix = "profadvisor/suggestion"
	}
	branch, err := nextBranch(ctx, opts.Dir, prefix)
	if err != nil {
		return nil, err
	}
	_, stderr, err = git(ctx, opts.Dir, nil, "checkout", "-b", branch)
	if err != nil {
		return nil, gitError("creating branch", err, stderr)
	}
	_, stderr, err = git(ctx, opts.Dir, strings.NewReader(d.Diff), "apply", "-")
	if err != nil {
		rollbackErr := rollback(ctx, opts.Dir, baseRef, branch)
		if rollbackErr != nil {
			return nil, fmt.Errorf("apply: applying diff: %w\n%s\napply: rollback failed: %v", err, stderr, rollbackErr)
		}
		return nil, gitError("applying diff", err, stderr)
	}

	// Stage only the files the patch touches, using "--" to prevent filenames
	// starting with a dash from being interpreted as flags.
	args := append([]string{"add", "--"}, res.FilesChanged...)
	_, stderr, err = git(ctx, opts.Dir, nil, args...)
	if err != nil {
		return nil, gitError("staging changes", err, stderr)
	}
	message := opts.Message
	if message == "" {
		subject := strings.SplitN(d.Change, "\n", 2)[0]
		message = "profadvisor: " + subject
	}
	body := d.Cause + "\n\nTarget: " + d.Target + "\nConfidence: " + d.Confidence
	_, stderr, err = git(ctx, opts.Dir, nil, "commit", "-m", message, "-m", body)
	if err != nil {
		return nil, gitError("committing changes", err, stderr)
	}
	commit, stderr, err := git(ctx, opts.Dir, nil, "rev-parse", "HEAD")
	if err != nil {
		return nil, gitError("recording commit", err, stderr)
	}
	res.Branch = branch
	res.Commit = strings.TrimSpace(commit)
	return res, nil
}

// git keeps every Git process cancellable and preserves stderr for callers;
// Git generally puts the useful explanation there, even for ordinary failures.
func git(ctx context.Context, dir string, stdin io.Reader, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	// Guarded, not assigned unconditionally: a nil *strings.Reader stored in
	// an io.Reader field is a non-nil interface holding a nil pointer, so
	// os/exec would start its stdin-copy goroutine and dereference it. This
	// line looks redundant and is not.
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func numstat(ctx context.Context, dir, diff string) ([]string, string, error) {
	stdout, stderr, err := git(ctx, dir, strings.NewReader(diff), "apply", "--numstat", "-")
	if err != nil {
		return nil, stderr, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) == 3 {
			files = append(files, fields[2])
		}
	}
	return files, stderr, nil
}

func nextBranch(ctx context.Context, dir, prefix string) (string, error) {
	for n := 1; ; n++ {
		branch := fmt.Sprintf("%s-%d", prefix, n)
		_, stderr, err := git(ctx, dir, nil, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
		if err == nil {
			continue
		}
		if stderr == "" {
			return branch, nil
		}
		return "", gitError("checking branch "+branch, err, stderr)
	}
}

func rollback(ctx context.Context, dir, baseRef, branch string) error {
	_, stderr, err := git(ctx, dir, nil, "checkout", baseRef)
	if err != nil {
		return gitError("returning to base branch", err, stderr)
	}
	_, stderr, err = git(ctx, dir, nil, "branch", "-D", branch)
	if err != nil {
		return gitError("deleting branch", err, stderr)
	}
	return nil
}

func changedFiles(status string) string {
	lines := strings.Split(strings.TrimSpace(status), "\n")
	if len(lines) > 5 {
		lines = append(lines[:5], "...")
	}
	return strings.Join(lines, "\n")
}

func gitError(action string, err error, stderr string) error {
	return fmt.Errorf("apply: %s: %w\n%s", action, err, stderr)
}
