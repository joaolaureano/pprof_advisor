// Package cmd is the CLI surface. It parses flags, calls internal packages, and
// decides what goes to stdout, what goes to stderr, and what the exit code is.
// It contains no analysis logic — that all lives under internal/, so it can be
// tested without a process.
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/joaolaureano/profadvisor/internal/render"
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
			"branch, and re-measures it.\n\n" +
			"The objective is chosen with --profile and --unit: CPU time (ns/op) by " +
			"default, or memory (B/op, allocs/op). A memory run keeps ns/op as a " +
			"guard, so a patch that saves bytes by spending time is rejected.\n\n" +
			"Every subcommand reports; only `verify` reaches a verdict, and it does " +
			"so from two benchmark outputs. `run` stops before judging and names the " +
			"verify invocation that follows it.\n\n" +
			"The result goes to stdout — JSON by default, or --format text to read " +
			"it yourself — and every diagnostic goes to stderr.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			format, err := cmd.Flags().GetString("format")
			if err != nil {
				return err
			}
			if format != "" && format != "json" && format != "text" {
				return fmt.Errorf("unknown --format %q (want json or text)", format)
			}
			return nil
		},
	}
	root.PersistentFlags().String("format", "json", "json (default, machine-readable) or text (human-readable)")
	root.AddCommand(newCaptureCmd(), newExtractCmd(), newPromptCmd(),
		newApplyCmd(), newVerifyCmd(), newEscapeCmd(), newBenchgenCmd())
	return root
}

// Execute runs the CLI and returns the process exit code.
//
// Exit 0 means the tool worked and the JSON on stdout is complete. Exit 1 means
// the tool failed. The verdict, if any, is in the JSON document, not the exit code.
func Execute() int { return execute(newRootCmd()) }

// execute is Execute with the root command injected, so a test can drive the
// real CLI — flag parsing, exit code, and the stdout/stderr split included —
// without a process. Diagnostics go to the command's own error stream rather
// than straight to os.Stderr for the same reason.
func execute(root *cobra.Command) int {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(root.ErrOrStderr(), "profadvisor:", err)
		return 1
	}
	return 0
}

// emit writes v to stdout according to the --format flag. Default is indented JSON.
// The --format flag is a persistent flag on the root command, so it's inherited
// by all subcommands.
func emit(cmd *cobra.Command, v any) error {
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}

	switch format {
	case "", "json":
		// Indented because a human reads this as often as a pipe does, and jq
		// is not always at hand.
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(v)
	case "text":
		s, err := render.Text(v)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), s)
		return err
	default:
		// Unreachable: PersistentPreRunE rejects this before any command runs,
		// which is the point — discovering a bad format here would mean
		// discovering it after a twenty-minute benchmark.
		return fmt.Errorf("unknown --format %q (want json or text)", format)
	}
}
