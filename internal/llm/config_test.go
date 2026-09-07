package llm_test

import (
	"strings"
	"testing"

	"github.com/joaolaureano/profadvisor/internal/llm"
	// Ensure providers are registered.
	_ "github.com/joaolaureano/profadvisor/internal/llm/providers"
)

func TestNewWithUnknownProvider(t *testing.T) {
	cfg := llm.Config{
		Provider: "unknown",
		APIKey:   "test-key",
	}
	_, err := llm.New(cfg)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}

	// Error should mention the provider name and list registered providers.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unknown") {
		t.Errorf("error should mention provider name: %s", errMsg)
	}
	if !strings.Contains(errMsg, "anthropic") {
		t.Errorf("error should list registered providers including anthropic: %s", errMsg)
	}
	if !strings.Contains(errMsg, "openai") {
		t.Errorf("error should list registered providers including openai: %s", errMsg)
	}
}

func TestProvidersReturnsRegistered(t *testing.T) {
	providers := llm.Providers()
	if len(providers) == 0 {
		t.Fatal("expected providers to be registered")
	}

	// Should be sorted.
	for i := 1; i < len(providers); i++ {
		if providers[i] < providers[i-1] {
			t.Errorf("providers not sorted: %v", providers)
		}
	}

	// Should contain our providers.
	hasAnthropic := false
	hasOpenAI := false
	for _, p := range providers {
		if p == "anthropic" {
			hasAnthropic = true
		}
		if p == "openai" {
			hasOpenAI = true
		}
	}
	if !hasAnthropic {
		t.Error("anthropic provider not registered")
	}
	if !hasOpenAI {
		t.Error("openai provider not registered")
	}
}
