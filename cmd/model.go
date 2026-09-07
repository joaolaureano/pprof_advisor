package cmd

import (
	"fmt"

	"github.com/joaolaureano/profadvisor/internal/analyze"
	"github.com/joaolaureano/profadvisor/internal/llm"
	_ "github.com/joaolaureano/profadvisor/internal/llm/providers"
	"github.com/joaolaureano/profadvisor/internal/prompt"
	"github.com/spf13/cobra"
)

// modelOptions is the flag set shared by the two subcommands that talk to a
// model. No default names a vendor: an empty value means "whatever the selected
// provider chooses", so the tool has no opinion about who answers.
type modelOptions struct {
	llm.Config
	promptsPath string
}

func modelFlags(c *cobra.Command, o *modelOptions) {
	f := c.Flags()
	f.StringVar(&o.Provider, "provider", "",
		"model provider: "+joinOr(llm.Providers())+" (default from PROFADVISOR_PROVIDER)")
	f.StringVar(&o.Model, "model", "", "model id (default: the provider's own)")
	f.StringVar(&o.BaseURL, "base-url", "", "API base URL, for a gateway or a local server")
	f.StringVar(&o.promptsPath, "prompts", "",
		"prompt catalog JSON to use instead of the built-in one")
}

// client builds the model client and the prompt catalog together, because both
// have to be valid before a capture is worth starting: this tool runs a
// multi-minute benchmark before it ever sends a request, and discovering a bad
// provider name or a malformed catalog afterwards wastes all of it.
func (o *modelOptions) client() (analyze.Client, *prompt.Catalog, error) {
	c, err := llm.New(o.Config)
	if err != nil {
		return nil, nil, err
	}
	cat, err := o.catalog()
	if err != nil {
		return nil, nil, err
	}
	return c, cat, nil
}

func (o *modelOptions) catalog() (*prompt.Catalog, error) {
	if o.promptsPath == "" {
		return prompt.Load()
	}
	return prompt.LoadFile(o.promptsPath)
}

func joinOr(names []string) string {
	switch len(names) {
	case 0:
		return "(none registered)"
	case 1:
		return names[0]
	}
	out := ""
	for i, n := range names {
		switch {
		case i == 0:
			out = n
		case i == len(names)-1:
			out += " or " + n
		default:
			out += ", " + n
		}
	}
	return out
}

// credentialsHelp is the same sentence on both subcommands that need a key.
func credentialsHelp() string {
	return fmt.Sprintf("Requires an API key: PROFADVISOR_API_KEY, or the selected "+
		"provider's own variable. Providers: %s.", joinOr(llm.Providers()))
}
