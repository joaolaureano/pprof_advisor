package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/joaolaureano/profadvisor/internal/prompt"
	"github.com/joaolaureano/profadvisor/internal/schema"
	"github.com/spf13/cobra"
)

func newPromptCmd() *cobra.Command {
	var (
		promptsPath string
		module      string
	)
	c := &cobra.Command{
		Use:   "prompt <extract.json>",
		Short: "Render the hotspots as a request for a language model",
		Long: "Renders an extract document as the two halves of a request — a system " +
			"prompt that states the objective and a user prompt carrying the hotspots " +
			"and their source — plus the JSON schema of the answer they ask for.\n\n" +
			"This command sends nothing. It opens no connection, reads no API key, and " +
			"names no vendor; it prints text. What to do with that text is yours: paste " +
			"it into a chat, or post it to whichever API you use. A diff that comes back " +
			"goes to `apply`, and `verify` decides whether it was worth it.\n\n" +
			"The wording lives in a JSON catalog embedded at build time. Pass --prompts " +
			"to render from a different one.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading extract: %w", err)
			}
			var r schema.ExtractResult
			if err := json.Unmarshal(raw, &r); err != nil {
				return fmt.Errorf("%s is not extract output: %w", args[0], err)
			}
			if r.SchemaVersion != schema.Version {
				return fmt.Errorf("%s has schema version %d, this build speaks %d",
					args[0], r.SchemaVersion, schema.Version)
			}
			cat, err := prompt.Load()
			if promptsPath != "" {
				cat, err = prompt.LoadFile(promptsPath)
			}
			if err != nil {
				return err
			}
			system, user, err := cat.Build(&r, module)
			if err != nil {
				return err
			}
			out, err := cat.ResponseSchema()
			if err != nil {
				return err
			}
			return emit(cmd, &schema.PromptResult{
				SchemaVersion:  schema.Version,
				System:         system,
				User:           user,
				ResponseSchema: out,
			})
		},
	}
	f := c.Flags()
	f.StringVar(&promptsPath, "prompts", "", "prompt catalog JSON to use instead of the built-in one")
	f.StringVar(&module, "module", "", "module path used to label source files")
	return c
}
