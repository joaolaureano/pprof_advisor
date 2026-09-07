package anthropic

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
	llm.Register("anthropic", newClient)
}

// defaultBaseURL is Anthropic's official API endpoint.
const defaultBaseURL = "https://api.anthropic.com"

// defaultModel is the default model for Anthropic.
const defaultModel = "claude-opus-5"

// newClient constructs an Anthropic client. It fails fast if the API key is missing.
func newClient(cfg llm.Config) (llm.Client, error) {
	// Fill in provider defaults.
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}

	// Determine API key: check PROFADVISOR_API_KEY first, fall back to ANTHROPIC_API_KEY.
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("llm: no API key: set PROFADVISOR_API_KEY or ANTHROPIC_API_KEY")
	}

	return &client{
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		apiKey:  apiKey,
	}, nil
}

// client implements llm.Client for Anthropic's Messages API.
type client struct {
	baseURL string
	model   string
	apiKey  string
}

// Complete sends a request to Anthropic and returns the response.
func (c *client) Complete(ctx context.Context, r llm.Request) (*llm.Response, error) {
	// If Model is specified in the request, use it; otherwise use the client's default.
	model := r.Model
	if model == "" {
		model = c.model
	}

	// Build the request body.
	body := map[string]any{
		"model":      model,
		"max_tokens": r.MaxTokens,
		"stream":     true,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": r.User,
			},
		},
	}

	// Add system prompt if provided, with cache_control for prompt caching.
	if r.System != "" {
		body["system"] = []map[string]any{
			{
				"type": "text",
				"text": r.System,
				"cache_control": map[string]any{
					"type": "ephemeral",
				},
			},
		}
	}

	// Add thinking if requested.
	if r.Thinking {
		body["thinking"] = map[string]any{
			"type": "adaptive",
		}
	}

	// Add output_config if effort or JSON schema is specified.
	if r.Effort != "" || r.JSONSchema != nil {
		outputConfig := make(map[string]any)
		if r.Effort != "" {
			outputConfig["effort"] = r.Effort
		}
		if r.JSONSchema != nil {
			outputConfig["format"] = map[string]any{
				"type":   "json_schema",
				"schema": r.JSONSchema,
			}
		}
		body["output_config"] = outputConfig
	}

	// Marshal the body.
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	// Create the HTTP request.
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("anthropic: new request: %w", err)
	}

	// Set headers.
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "text/event-stream")

	// Send the request.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check the status code.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read the body (up to 8 KB) for the error message.
		respBody := io.LimitReader(resp.Body, 8*1024)
		bodyText, _ := io.ReadAll(respBody)
		return nil, fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, string(bodyText))
	}

	// Parse the SSE stream.
	response := &llm.Response{}
	err = llm.ScanEvents(resp.Body, func(data []byte) error {
		var event map[string]any
		if err := json.Unmarshal(data, &event); err != nil {
			return fmt.Errorf("anthropic: parse event: %w", err)
		}

		eventType, ok := event["type"].(string)
		if !ok {
			return nil
		}

		switch eventType {
		case "content_block_delta":
			delta, ok := event["delta"].(map[string]any)
			if !ok {
				break
			}
			if deltaType, ok := delta["type"].(string); ok && deltaType == "text_delta" {
				if text, ok := delta["text"].(string); ok {
					response.Text += text
				}
			}

		case "message_start":
			message, ok := event["message"].(map[string]any)
			if !ok {
				break
			}
			usage, ok := message["usage"].(map[string]any)
			if !ok {
				break
			}
			if inputTokens, ok := usage["input_tokens"].(float64); ok {
				response.InputTokens = int64(inputTokens)
			}

		case "message_delta":
			// Stop reason.
			if delta, ok := event["delta"].(map[string]any); ok {
				if stopReason, ok := delta["stop_reason"].(string); ok {
					response.StopReason = stopReason
				}
				// Check for refusal explanation.
				if stopReason, ok := delta["stop_reason"].(string); ok && stopReason == "refusal" {
					if stopDetails, ok := delta["stop_details"].(map[string]any); ok {
						if explanation, ok := stopDetails["explanation"].(string); ok {
							response.Refusal = explanation
						}
					}
				}
			}
			// Output tokens.
			if usage, ok := event["usage"].(map[string]any); ok {
				if outputTokens, ok := usage["output_tokens"].(float64); ok {
					response.OutputTokens = int64(outputTokens)
				}
			}

		case "error":
			errObj, ok := event["error"].(map[string]any)
			if !ok {
				break
			}
			errType, _ := errObj["type"].(string)
			errMsg, _ := errObj["message"].(string)
			return fmt.Errorf("anthropic: API error (%s): %s", errType, errMsg)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}
