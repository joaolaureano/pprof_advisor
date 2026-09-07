package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/joaolaureano/profadvisor/internal/llm"
)

func init() {
	llm.Register("openai", newClient)
}

// defaultBaseURL is OpenAI's official API endpoint.
const defaultBaseURL = "https://api.openai.com"

// defaultModel is the default model for OpenAI.
const defaultModel = "gpt-5"

// newClient constructs an OpenAI client. It fails fast if the API key is missing.
func newClient(cfg llm.Config) (llm.Client, error) {
	// Fill in provider defaults.
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}

	// Determine API key: check PROFADVISOR_API_KEY first, fall back to OPENAI_API_KEY.
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("llm: no API key: set PROFADVISOR_API_KEY or OPENAI_API_KEY")
	}

	return &client{
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		apiKey:  apiKey,
	}, nil
}

// client implements llm.Client for OpenAI's Chat Completions API.
type client struct {
	baseURL string
	model   string
	apiKey  string
}

// Complete sends a request to OpenAI and returns the response.
func (c *client) Complete(ctx context.Context, r llm.Request) (*llm.Response, error) {
	// If Model is specified in the request, use it; otherwise use the client's default.
	model := r.Model
	if model == "" {
		model = c.model
	}

	// Build messages array.
	messages := make([]map[string]string, 0, 2)

	// Add system message if provided.
	if r.System != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": r.System,
		})
	}

	// Add user message.
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": r.User,
	})

	// Build the request body.
	body := map[string]any{
		"model":                 model,
		"max_completion_tokens": r.MaxTokens,
		"stream":                true,
		"stream_options": map[string]any{
			"include_usage": true,
		},
		"messages": messages,
	}

	// Add response format for JSON schema if specified.
	if r.JSONSchema != nil {
		body["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "diagnosis",
				"strict": true,
				"schema": r.JSONSchema,
			},
		}
	}

	// Add reasoning effort if specified.
	// Note: Thinking is ignored for OpenAI; this provider has no equivalent knob.
	if r.Effort != "" {
		body["reasoning_effort"] = r.Effort
	}

	// Marshal the body.
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	// Create the HTTP request.
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("openai: new request: %w", err)
	}

	// Set headers.
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "text/event-stream")

	// Send the request.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check the status code.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read the body (up to 8 KB) for the error message.
		respBody := io.LimitReader(resp.Body, 8*1024)
		bodyText, _ := io.ReadAll(respBody)
		return nil, fmt.Errorf("openai: HTTP %d: %s", resp.StatusCode, string(bodyText))
	}

	// Parse the SSE stream.
	response := &llm.Response{}
	err = llm.ScanEvents(resp.Body, func(data []byte) error {
		var event map[string]any
		if err := json.Unmarshal(data, &event); err != nil {
			return fmt.Errorf("openai: parse event: %w", err)
		}

		// Extract content from choices[0].
		choices, ok := event["choices"].([]any)
		if !ok || len(choices) == 0 {
			return nil
		}

		choice, ok := choices[0].(map[string]any)
		if !ok {
			return nil
		}

		// Extract delta content and refusal.
		delta, ok := choice["delta"].(map[string]any)
		if ok {
			if content, ok := delta["content"].(string); ok {
				response.Text += content
			}
			if refusal, ok := delta["refusal"].(string); ok {
				response.Refusal += refusal
			}
		}

		// Extract finish_reason.
		if finishReason, ok := choice["finish_reason"]; ok && finishReason != nil {
			if finishReasonStr, ok := finishReason.(string); ok {
				response.StopReason = finishReasonStr
			}
		}

		// Extract usage if present.
		if usage, ok := event["usage"].(map[string]any); ok {
			if promptTokens, ok := usage["prompt_tokens"].(float64); ok {
				response.InputTokens = int64(promptTokens)
			}
			if completionTokens, ok := usage["completion_tokens"].(float64); ok {
				response.OutputTokens = int64(completionTokens)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}
