package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joaolaureano/profadvisor/internal/llm"
)

func TestCompleteParsesSSEStream(t *testing.T) {
	// Create a mock SSE stream response.
	sseStream := `data: {"type":"message_start","message":{"id":"msg_123","usage":{"input_tokens":10}}}
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" "}}
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"world"}}
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}
data: [DONE]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(200)
		w.Write([]byte(sseStream))
	}))
	defer server.Close()

	// Create client pointing to test server.
	cfg := llm.Config{
		Provider: "anthropic",
		Model:    "claude-test",
		BaseURL:  server.URL,
		APIKey:   "test-key",
	}
	client, err := newClient(cfg)
	if err != nil {
		t.Fatalf("newClient failed: %v", err)
	}

	// Call Complete.
	resp, err := client.Complete(context.Background(), llm.Request{
		User: "Hello",
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Verify response.
	if resp.Text != "Hello world" {
		t.Errorf("got text %q, want %q", resp.Text, "Hello world")
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("got stop_reason %q, want %q", resp.StopReason, "end_turn")
	}
	if resp.InputTokens != 10 {
		t.Errorf("got input_tokens %d, want 10", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("got output_tokens %d, want 5", resp.OutputTokens)
	}
}

func TestError400ResponseIncludesBody(t *testing.T) {
	// Create a server that returns a 400 error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"Invalid model"}}`))
	}))
	defer server.Close()

	cfg := llm.Config{
		Provider: "anthropic",
		BaseURL:  server.URL,
		APIKey:   "test-key",
	}
	client, err := newClient(cfg)
	if err != nil {
		t.Fatalf("newClient failed: %v", err)
	}

	_, err = client.Complete(context.Background(), llm.Request{
		User: "test",
	})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	errMsg := err.Error()
	if !bytes.Contains([]byte(errMsg), []byte("400")) {
		t.Errorf("error should contain status code: %s", errMsg)
	}
	if !bytes.Contains([]byte(errMsg), []byte("Invalid model")) {
		t.Errorf("error should contain response body: %s", errMsg)
	}
}

func TestRequestBodyFormat(t *testing.T) {
	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the request body.
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
		w.Write([]byte(`data: [DONE]`))
	}))
	defer server.Close()

	cfg := llm.Config{
		Provider: "anthropic",
		BaseURL:  server.URL,
		APIKey:   "test-key",
	}
	client, err := newClient(cfg)
	if err != nil {
		t.Fatalf("newClient failed: %v", err)
	}

	// Send a request with all options.
	schema := map[string]any{"type": "object"}
	_, err = client.Complete(context.Background(), llm.Request{
		System:     "You are helpful",
		User:       "Test",
		MaxTokens:  100,
		JSONSchema: schema,
		Effort:     "high",
		Thinking:   true,
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Verify request body format.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse captured body: %v", err)
	}

	// Check system message has cache_control.
	system, ok := body["system"].([]any)
	if !ok || len(system) == 0 {
		t.Fatal("system should be an array")
	}
	systemMsg := system[0].(map[string]any)
	cacheControl := systemMsg["cache_control"].(map[string]any)
	if cacheControl["type"] != "ephemeral" {
		t.Errorf("cache_control type should be ephemeral, got %v", cacheControl["type"])
	}

	// Check thinking.
	thinking, ok := body["thinking"].(map[string]any)
	if !ok {
		t.Fatal("thinking should be present")
	}
	if thinking["type"] != "adaptive" {
		t.Errorf("thinking type should be adaptive, got %v", thinking["type"])
	}

	// Check output_config with effort and schema.
	outputConfig, ok := body["output_config"].(map[string]any)
	if !ok {
		t.Fatal("output_config should be present")
	}
	if outputConfig["effort"] != "high" {
		t.Errorf("effort should be high, got %v", outputConfig["effort"])
	}
	format, ok := outputConfig["format"].(map[string]any)
	if !ok {
		t.Fatal("format should be present in output_config")
	}
	if format["type"] != "json_schema" {
		t.Errorf("format type should be json_schema, got %v", format["type"])
	}
}

func TestMissingAPIKeyError(t *testing.T) {
	// Clear both API key env vars to force the error.
	t.Setenv("PROFADVISOR_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	cfg := llm.Config{Provider: "anthropic"}
	_, err := newClient(cfg)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}

	errMsg := err.Error()
	if !bytes.Contains([]byte(errMsg), []byte("PROFADVISOR_API_KEY")) {
		t.Errorf("error should mention PROFADVISOR_API_KEY: %s", errMsg)
	}
	if !bytes.Contains([]byte(errMsg), []byte("ANTHROPIC_API_KEY")) {
		t.Errorf("error should mention ANTHROPIC_API_KEY: %s", errMsg)
	}
}
