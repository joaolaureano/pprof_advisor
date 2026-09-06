// Package analyze turns extract output into a diagnosis and a concrete patch by
// asking Claude.
//
// The package owns the request shape but not the judgement: it does not decide
// whether a suggestion is good. That is what `verify` is for, and treating the
// model's confidence as evidence would defeat the point of the pipeline.
package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

// DefaultModel is the model used unless overridden. Performance analysis is
// exactly the kind of work where the strongest model earns its cost: the output
// is a patch that a benchmark will accept or reject, so a weak suggestion is not
// cheap, it is a wasted capture-apply-verify cycle.
const DefaultModel = "claude-opus-5"

// Options configures a single analysis request.
type Options struct {
	// Model overrides DefaultModel.
	Model string
	// Module is the target repository's module path, included in the prompt so
	// the model writes diff paths relative to the right root. Optional.
	Module string
	// MaxTokens bounds the response. Zero means 16000.
	MaxTokens int64
	// Effort maps to output_config.effort. Empty means "xhigh", which is the
	// right default for this workload: the answer is a patch, and a shallow
	// answer costs a whole benchmark cycle to discover.
	Effort anthropic.OutputConfigEffort
}

// Client is the subset of the SDK this package uses, so tests can substitute a
// canned response without a network call or an API key.
type Client interface {
	NewMessage(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error)
}

type sdkClient struct{ c anthropic.Client }

func (s sdkClient) NewMessage(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
	// Streaming, not a plain create: max_tokens is large enough that a
	// non-streaming request can outlive the HTTP timeout on a slow answer.
	stream := s.c.Messages.NewStreaming(ctx, params)
	var msg anthropic.Message
	for stream.Next() {
		if err := msg.Accumulate(stream.Current()); err != nil {
			return nil, fmt.Errorf("accumulating stream: %w", err)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	return &msg, nil
}

// NewClient returns a Client backed by the Anthropic SDK. Credentials are
// resolved by the SDK (ANTHROPIC_API_KEY, or an `ant auth login` profile).
func NewClient() Client {
	return sdkClient{c: anthropic.NewClient()}
}

// responseSchema is the JSON schema the model's answer is constrained to.
// Constraining the shape is what lets `analyze | apply` be a pipe rather than a
// parser with a prayer in it.
var responseSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"target", "cause", "change", "diff", "confidence", "risks"},
	"properties": map[string]any{
		"target": map[string]any{
			"type":        "string",
			"description": "Fully qualified name of the function being optimized, as it appears in the profile.",
		},
		"cause": map[string]any{
			"type":        "string",
			"description": "Why this code is hot, citing the specific lines and cost figures from the profile, in the profile's own unit.",
		},
		"change": map[string]any{
			"type":        "string",
			"description": "What the patch does, in two or three sentences, and the mechanism by which it should improve the objective metric.",
		},
		"diff": map[string]any{
			"type":        "string",
			"description": "A unified diff applying cleanly with 'git apply' from the repository root, paths relative to that root, at least 3 lines of context.",
		},
		"confidence": map[string]any{
			"type":        "string",
			"enum":        []string{"high", "medium", "low"},
			"description": "How likely this change is to produce a statistically significant improvement in the objective metric.",
		},
		"risks": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "Behavioural changes this diff could plausibly cause. Empty if none.",
		},
	},
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
	model := opts.Model
	if model == "" {
		model = DefaultModel
	}
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 16000
	}
	effort := opts.Effort
	if effort == "" {
		effort = anthropic.OutputConfigEffortXhigh
	}

	adaptive := anthropic.ThinkingConfigAdaptiveParam{}
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		System: []anthropic.TextBlockParam{{
			Text: systemPrompt(cfg),
			// The system prompt and schema are byte-identical across every
			// hotspot in a run, so caching them is free savings on the
			// second and later calls.
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		Thinking: anthropic.ThinkingConfigParamUnion{OfAdaptive: &adaptive},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: effort,
			Format: anthropic.JSONOutputFormatParam{Schema: responseSchema},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(buildUserPrompt(r, opts.Module))),
		},
	}

	msg, err := c.NewMessage(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}
	if msg.StopReason == anthropic.StopReasonRefusal {
		return nil, fmt.Errorf("analyze: model declined (%s): %s",
			msg.StopDetails.Category, msg.StopDetails.Explanation)
	}

	var text strings.Builder
	for _, block := range msg.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	raw := strings.TrimSpace(text.String())
	if raw == "" {
		return nil, fmt.Errorf("analyze: model returned no text (stop_reason %q)", msg.StopReason)
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
		Model:         model,
		Measurement:   cfg,
		Target:        out.Target,
		Cause:         out.Cause,
		Change:        out.Change,
		Diff:          out.Diff,
		Confidence:    out.Confidence,
		Risks:         out.Risks,
	}, nil
}
