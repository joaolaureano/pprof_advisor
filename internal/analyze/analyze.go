// Package analyze turns extract output into a diagnosis and a concrete patch by
// asking a language model.
//
// The package owns the request shape but not the judgement: it does not decide
// whether a suggestion is good. That is what `verify` is for, and treating the
// model's confidence as evidence would defeat the point of the pipeline.
//
// Nothing here names a vendor. The request is built in the neutral types of
// internal/llm and the wording comes from internal/prompt, so swapping the
// provider is a flag rather than an edit.
package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/joaolaureano/profadvisor/internal/llm"
	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/prompt"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

// Options configures a single analysis request.
type Options struct {
	// Model overrides the provider's default model.
	Model string
	// Provider names the model provider, recorded on the diagnosis so a diff
	// can be traced back to what produced it. Informational here; the caller
	// has already constructed the Client.
	Provider string
	// Module is the target repository's module path, included in the prompt so
	// the model writes diff paths relative to the right root. Optional.
	Module string
	// MaxTokens bounds the response. Zero means 16000.
	MaxTokens int64
	// Effort is the reasoning-effort hint. Empty means "xhigh", which is the
	// right default for this workload: the answer is a patch, and a shallow
	// answer costs a whole benchmark cycle to discover. A provider without an
	// equivalent knob ignores it.
	Effort string
	// Prompts overrides the built-in prompt catalog. Nil loads the embedded one.
	Prompts *prompt.Catalog
}

// Client is the model client this package needs. It is an alias rather than a
// second declaration so a caller can pass any llm.Client straight through.
type Client = llm.Client

// responseSchema is the JSON schema the model's answer is constrained to.
// Constraining the shape is what lets `analyze | apply` be a pipe rather than a
// parser with a prayer in it. The field descriptions live in the prompt catalog
// with the rest of the wording.
func responseSchema(cat *prompt.Catalog) (map[string]any, error) {
	str := func(key string) (map[string]any, error) {
		d, err := cat.Render(key, map[string]string{})
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "string", "description": d}, nil
	}
	props := map[string]any{}
	for field, key := range map[string]string{
		"target": "schema.target", "cause": "schema.cause",
		"change": "schema.change", "diff": "schema.diff",
	} {
		p, err := str(key)
		if err != nil {
			return nil, err
		}
		props[field] = p
	}
	conf, err := str("schema.confidence")
	if err != nil {
		return nil, err
	}
	conf["enum"] = []string{"high", "medium", "low"}
	props["confidence"] = conf

	risks, err := cat.Render("schema.risks", map[string]string{})
	if err != nil {
		return nil, err
	}
	props["risks"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": risks,
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"target", "cause", "change", "diff", "confidence", "risks"},
		"properties":           props,
	}, nil
}

// Run asks the model for one optimization proposal.
func Run(ctx context.Context, c Client, r *schema.ExtractResult, opts Options) (*schema.Diagnosis, error) {
	if r == nil || len(r.Hotspots) == 0 {
		return nil, errors.New("analyze: extract result has no hotspots")
	}
	// The objective travels in the extract document rather than in Options:
	// it is a property of the profile that was captured, and letting a caller
	// pass a different one here would produce a diagnosis about a metric the
	// hotspots were never ranked by.
	cfg := r.Profile.Measurement
	if (cfg == measurement.Config{}) {
		resolved, err := measurement.Resolve("", "")
		if err != nil {
			return nil, err
		}
		cfg = resolved
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}

	cat := opts.Prompts
	if cat == nil {
		loaded, err := prompt.Load()
		if err != nil {
			return nil, fmt.Errorf("analyze: %w", err)
		}
		cat = loaded
	}

	system, err := systemPrompt(cat, cfg)
	if err != nil {
		return nil, fmt.Errorf("analyze: system prompt: %w", err)
	}
	user, err := buildUserPrompt(cat, r, opts.Module)
	if err != nil {
		return nil, fmt.Errorf("analyze: user prompt: %w", err)
	}
	outSchema, err := responseSchema(cat)
	if err != nil {
		return nil, fmt.Errorf("analyze: response schema: %w", err)
	}

	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 16000
	}
	effort := opts.Effort
	if effort == "" {
		effort = "xhigh"
	}

	resp, err := c.Complete(ctx, llm.Request{
		Model:      opts.Model,
		System:     system,
		User:       user,
		MaxTokens:  maxTokens,
		JSONSchema: outSchema,
		Effort:     effort,
		Thinking:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}
	if resp.Refusal != "" {
		return nil, fmt.Errorf("analyze: model declined: %s", resp.Refusal)
	}

	raw := strings.TrimSpace(resp.Text)
	if raw == "" {
		return nil, fmt.Errorf("analyze: model returned no text (stop_reason %q)", resp.StopReason)
	}

	var out struct {
		Target     string   `json:"target"`
		Cause      string   `json:"cause"`
		Change     string   `json:"change"`
		Diff       string   `json:"diff"`
		Confidence string   `json:"confidence"`
		Risks      []string `json:"risks"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("analyze: response was not the requested JSON: %w", err)
	}
	if strings.TrimSpace(out.Diff) == "" {
		return nil, errors.New("analyze: model returned an empty diff")
	}

	return &schema.Diagnosis{
		SchemaVersion: schema.Version,
		Provider:      opts.Provider,
		Model:         opts.Model,
		Measurement:   cfg,
		Target:        out.Target,
		Cause:         out.Cause,
		Change:        out.Change,
		Diff:          out.Diff,
		Confidence:    out.Confidence,
		Risks:         out.Risks,
	}, nil
}
