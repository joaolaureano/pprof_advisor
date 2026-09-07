package openai

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
	sseStream := `data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}
data: {"choices":[{"delta":{"content":" "},"finish_reason":null}]}
data: {"choices":[{"delta":{"content":"world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}
data: [DONE]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(200)
		w.Write([]byte(sseStream))
	}))
	defer server.Close()

	// Create client pointing to test server.
	cfg := llm.Config{
		Provider: "openai",
		Model:    "gpt-test",
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
	if resp.StopReason != "stop" {
		t.Errorf("got stop_reason %q, want %q", resp.StopReason, "stop")
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
		w.Write([]byte(`{"error":{"message":"Invalid model specified"}}`))
	}))
	defer server.Close()

	cfg := llm.Config{
		Provider: "openai",
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
		Provider: "openai",
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
		Thinking:   true, // This should be ignored for OpenAI.
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Verify request body format.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse captured body: %v", err)
	}

	// Check response format has strict: true.
	responseFormat, ok := body["response_format"].(map[string]any)
	if !ok {
		t.Fatal("response_format should be present")
	}
	jsonSchema, ok := responseFormat["json_schema"].(map[string]any)
	if !ok {
		t.Fatal("json_schema should be present in response_format")
	}
	if strict, ok := jsonSchema["strict"].(bool); !ok || !strict {
		t.Errorf("strict should be true, got %v", jsonSchema["strict"])
	}

	// Check reasoning_effort is set.
	if effort, ok := body["reasoning_effort"].(string); !ok || effort != "high" {
		t.Errorf("reasoning_effort should be high, got %v", body["reasoning_effort"])
	}

	// Verify Thinking is not in the request (OpenAI doesn't support it).
	if _, ok := body["thinking"]; ok {
		t.Error("thinking should not be in OpenAI request")
	}
}

func TestMissingAPIKeyError(t *testing.T) {
	// Clear both API key env vars to force the error.
	t.Setenv("PROFADVISOR_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	cfg := llm.Config{Provider: "openai"}
	_, err := newClient(cfg)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}

	errMsg := err.Error()
	if !bytes.Contains([]byte(errMsg), []byte("PROFADVISOR_API_KEY")) {
		t.Errorf("error should mention PROFADVISOR_API_KEY: %s", errMsg)
	}
	if !bytes.Contains([]byte(errMsg), []byte("OPENAI_API_KEY")) {
		t.Errorf("error should mention OPENAI_API_KEY: %s", errMsg)
	}
}
