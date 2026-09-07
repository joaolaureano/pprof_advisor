package cmd

import (
	"fmt"
	"os"

	"github.com/joaolaureano/profadvisor/internal/apply"
	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	var opts apply.Options
	c := &cobra.Command{
		Use:   "apply <patch.diff>",
		Short: "Apply a unified diff on a new branch",
		Long: "Applies a unified diff on a fresh branch, never on the branch you are " +
			"working on. The diff is a plain patch file, whatever produced it.\n\n" +
			"The command refuses to run against a dirty working tree, and if the diff " +
			"fails to apply after the branch was created it deletes the branch and " +
			"returns you to where you started. A patch that turns out to be wrong " +
			"costs you a branch, not your work.\n\n" +
			"Reads standard input when the path is `-`.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				raw []byte
				err error
			)
			if args[0] == "-" {
				raw, err = os.ReadFile(os.Stdin.Name())
			} else {
				raw, err = os.ReadFile(args[0])
			}
			if err != nil {
				return fmt.Errorf("reading patch: %w", err)
			}
			res, err := apply.Run(cmd.Context(), string(raw), opts)
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
