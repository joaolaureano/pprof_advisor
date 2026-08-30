package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/joaolaureano/profadvisor/internal/analyze"
	"github.com/joaolaureano/profadvisor/internal/schema"
	"github.com/spf13/cobra"
)

func newAnalyzeCmd() *cobra.Command {
	var opts analyze.Options
	c := &cobra.Command{
		Use:   "analyze <extract.json>",
		Short: "Ask Claude what to do about the hot path",
		Long: "Reads the output of `extract` and returns a diagnosis and a unified " +
			"diff.\n\nThe answer is a proposal, not a conclusion: nothing here has " +
			"been measured. Run `apply` and then `verify` before believing it.\n\n" +
			"Requires ANTHROPIC_API_KEY, or credentials from `ant auth login`.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading extract output: %w", err)
			}
			var in schema.ExtractResult
			if err := json.Unmarshal(raw, &in); err != nil {
				return fmt.Errorf("%s is not extract output: %w", args[0], err)
			}
			if in.SchemaVersion != schema.Version {
				return fmt.Errorf("%s has schema version %d, this build speaks %d",
					args[0], in.SchemaVersion, schema.Version)
			}
			res, err := analyze.Run(cmd.Context(), analyze.NewClient(), &in, opts)
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	f := c.Flags()
	f.StringVar(&opts.Model, "model", analyze.DefaultModel, "model id")
	f.StringVar(&opts.Module, "module", "", "target repository module path, for diff paths")
	f.Int64Var(&opts.MaxTokens, "max-tokens", 16000, "response cap")
	return c
}
