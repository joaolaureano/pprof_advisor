// Package cmd is the CLI surface. It parses flags, calls internal packages, and
// decides what goes to stdout, what goes to stderr, and what the exit code is.
// It contains no analysis logic — that all lives under internal/, so it can be
// tested without a process.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// The I/O contract, which AGENTS.md documents and agents depend on:
// structured results go to stdout as JSON and nothing else ever does; every
// diagnostic goes to stderr; a non-zero exit means the JSON on stdout is
// absent or incomplete.

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "profadvisor",
		Short: "Find, fix, and prove hot-paths in Go benchmarks",
		Long: "profadvisor captures a profile from a Go benchmark, asks a language " +
			"model what to do about the hot path, applies the suggestion on a " +
			"branch, and " +
			"re-measures to decide whether it actually helped.\n\n" +
			"The objective is chosen with --profile and --unit: CPU time (ns/op) by " +
			"default, or memory (B/op, allocs/op). A memory run keeps ns/op as a " +
			"guard, so a patch that saves bytes by spending time is rejected.\n\n" +
			"Every subcommand writes JSON to stdout and diagnostics to stderr.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newCaptureCmd(), newExtractCmd(), newAnalyzeCmd(),
		newApplyCmd(), newVerifyCmd(), newRunCmd())
	return root
}

// Execute runs the CLI and returns the process exit code.
//
// Exit 2 is reserved for "the tool worked and the answer was bad" — a verified
// regression. It is separated from exit 1 so a CI job can tell a failed
// optimization apart from a broken invocation.
func Execute() int { return execute(newRootCmd()) }

// execute is Execute with the root command injected, so a test can drive the
// real CLI — flag parsing, exit code, and the stdout/stderr split included —
// without a process. Diagnostics go to the command's own error stream rather
// than straight to os.Stderr for the same reason.
func execute(root *cobra.Command) int {
	if err := root.Execute(); err != nil {
		out := root.ErrOrStderr()
		if errors.Is(err, errRegressed) {
			fmt.Fprintln(out, "profadvisor: the objective regressed; the change was measured and rejected")
			return 2
		}
		fmt.Fprintln(out, "profadvisor:", err)
		return 1
	}
	return 0
}

// emit writes v to stdout as indented JSON. Indented because a human reads this
// as often as a pipe does, and jq is not always at hand.
func emit(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
