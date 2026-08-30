// Package cmd is the CLI surface. It parses flags, calls internal packages, and
// decides what goes to stdout, what goes to stderr, and what the exit code is.
// It contains no analysis logic — that all lives under internal/, so it can be
// tested without a process.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// The I/O contract, which AGENTS.md documents and agents depend on:
// structured results go to stdout as JSON and nothing else ever does; every
// diagnostic goes to stderr; a non-zero exit means the JSON on stdout is
// absent or incomplete.

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "profadvisor",
		Short: "Find, fix, and prove CPU hot-paths in Go benchmarks",
		Long: "profadvisor captures a CPU profile from a Go benchmark, asks Claude " +
			"what to do about the hot path, applies the suggestion on a branch, and " +
			"re-measures to decide whether it actually helped.\n\n" +
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
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		if errors.Is(err, errRegressed) {
			fmt.Fprintln(os.Stderr, "profadvisor: the change made things slower")
			return 2
		}
		fmt.Fprintln(os.Stderr, "profadvisor:", err)
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
