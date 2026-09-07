// Package llm provides a vendor-neutral language model client interface.
//
// The Client interface describes what this tool needs of a model, not what any
// one API offers, so switching providers is configuration rather than an edit.
// Request and Response carry metadata that providers use differently — fields
// like Effort and Thinking are hints, not requirements. A provider with no
// equivalent drops the hint silently rather than erroring; this tool does not
// fail the whole run because one model type doesn't have extended reasoning.
package llm

import "context"

// Request is the input to Complete.
type Request struct {
	// Model is the provider's model identifier. Empty means the provider's
	// own default, so no vendor's naming leaks into the core.
	Model string

	// System is a system prompt, cached by providers that support it.
	// Non-empty System will be sent as a system message in the API request.
	System string

	// User is the user message text.
	User string

	// MaxTokens is the maximum number of tokens to generate. If zero, the
	// provider's default is used.
	MaxTokens int64

	// JSONSchema constrains the response format. When non-nil, the provider
	// must ensure the response conforms to this JSON schema. Nil means the
	// response is free-form text.
	JSONSchema map[string]any

	// Effort is a hint for extended thinking capabilities, one of "low",
	// "medium", "high", or "xhigh". Empty string means the provider's default.
	// A provider with no equivalent knob ignores this silently.
	Effort string

	// Thinking requests extended reasoning where available. A provider with
	// no equivalent ignores this silently.
	Thinking bool
}

// Response is the output from Complete.
type Response struct {
	// Text is the concatenated model output.
	Text string

	// StopReason explains why generation stopped, one of "end_turn",
	// "max_tokens", "refusal", or a provider-specific value.
	StopReason string

	// Refusal is non-empty when the model declined to answer. It carries
	// the model's explanation of why.
	Refusal string

	// InputTokens is the count of tokens in the input (including system prompt).
	InputTokens int64

	// OutputTokens is the count of tokens in the generated response.
	OutputTokens int64
}

// Client is the interface that every LLM provider must implement.
type Client interface {
	// Complete sends a request to the model and returns the response.
	// The context is passed to the underlying HTTP request, so callers
	// can set timeouts or deadlines.
	Complete(ctx context.Context, r Request) (*Response, error)
}
