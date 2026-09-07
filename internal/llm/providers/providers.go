// Package providers blank-imports every built-in provider so that importing it
// registers them. Kept separate from llm itself to avoid an import cycle.
package providers

import (
	"github.com/joaolaureano/profadvisor/internal/llm"

	_ "github.com/joaolaureano/profadvisor/internal/llm/anthropic"
	_ "github.com/joaolaureano/profadvisor/internal/llm/openai"
)

func init() {
	// The out-of-the-box choice. This package is the one place that names
	// vendors, so it is also the one place that gets to pick a default;
	// PROFADVISOR_PROVIDER or --provider overrides it.
	llm.DefaultProvider = "anthropic"
}
