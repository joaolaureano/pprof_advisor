package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/joaolaureano/profadvisor/internal/apply"
	"github.com/joaolaureano/profadvisor/internal/schema"
	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	var opts apply.Options
	c := &cobra.Command{
		Use:   "apply <diagnosis.json>",
		Short: "Apply the suggested diff on a new branch",
		Long: "Applies the diff from `analyze` on a fresh branch, never on the branch " +
			"you are working on.\n\nThe command refuses to run against a dirty working " +
			"tree, and if the diff fails to apply after the branch was created it " +
			"deletes the branch and returns you to where you started. A suggestion " +
			"that turns out to be wrong should cost you a branch, not your work.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading diagnosis: %w", err)
			}
			var d schema.Diagnosis
			if err := json.Unmarshal(raw, &d); err != nil {
				return fmt.Errorf("%s is not analyze output: %w", args[0], err)
			}
			res, err := apply.Run(cmd.Context(), &d, opts)
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	f := c.Flags()
	f.StringVar(&opts.Dir, "dir", ".", "target repository root")
	f.StringVar(&opts.BranchPrefix, "branch-prefix", "profadvisor/suggestion", "branch namespace")
	f.StringVar(&opts.Message, "message", "", "override the commit message")
	f.BoolVar(&opts.DryRun, "dry-run", false, "check that the diff applies, then change nothing")
	return c
}
