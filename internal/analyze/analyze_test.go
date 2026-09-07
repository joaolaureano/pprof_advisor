package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/joaolaureano/profadvisor/internal/extract"
	"github.com/joaolaureano/profadvisor/internal/fixture"
	"github.com/joaolaureano/profadvisor/internal/llm"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

// fakeClient records the request and returns a canned response, so the prompt
// and the request shape can be tested without an API key or a network call.
type fakeClient struct {
	got   llm.Request
	reply string
	err   error
}

func (f *fakeClient) Complete(_ context.Context, r llm.Request) (*llm.Response, error) {
	f.got = r
	if f.err != nil {
		return nil, f.err
	}
	return &llm.Response{Text: f.reply, StopReason: "end_turn"}, nil
}

func realExtract(t *testing.T) *schema.ExtractResult {
	t.Helper()
	p, err := fixture.Load("all.prof")
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	r, err := extract.FromProfile(p, "all.prof", extract.Options{TopN: 3})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return r
}

const goodReply = `{"target":"matcher.(*Table).Match","cause":"linear scan","change":"index the patterns","diff":"--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-a\n+b\n","confidence":"medium","risks":["ordering"]}`

func TestPromptCarriesTheProfileEvidence(t *testing.T) {
	r := realExtract(t)
	f := &fakeClient{reply: goodReply}
	if _, err := Run(context.Background(), f, r, Options{Module: "example.com/profadvisor/fixture"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	prompt := f.got.User

	// The whole reason extract filters runtime frames is so the model reasons
	// about the code under test. If the prompt lost the numbers or the source,
	// the model is guessing and the pipeline is theatre.
	for _, want := range []string{
		"matcher.",                        // the real hotspot made it in
		"example.com/profadvisor/fixture", // the module was named
		"self",                            // per-line cost annotations are present
		"```go",                           // source was actually attached
		"filtered out of the ranking",     // the model is told what was removed
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
	// Scoped to the hotspot section on purpose. Runtime frames are allowed —
	// wanted, even — in the "where the filtered cost went" block above it,
	// because that is what explains the cost. What must never happen is a
	// runtime frame appearing as something to edit.
	_, hotspots, found := strings.Cut(prompt, "# Hotspots, hottest first")
	if !found {
		t.Fatal("prompt has no hotspot section")
	}
	if strings.Contains(hotspots, "runtime.") {
		t.Error("a filtered runtime frame was ranked as a hotspot")
	}
	if !strings.Contains(prompt, "Where the filtered cost went") {
		t.Error("the filtered cost was reported as a total but not attributed")
	}
}

func TestRequestShape(t *testing.T) {
	r := realExtract(t)
	f := &fakeClient{reply: goodReply}
	if _, err := Run(context.Background(), f, r, Options{Model: "test-model"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if f.got.Model != "test-model" {
		t.Errorf("model = %q, want %q", f.got.Model, "test-model")
	}
	if !f.got.Thinking {
		t.Error("extended reasoning was not requested")
	}
	if f.got.JSONSchema == nil {
		t.Error("no output schema was sent; the response shape would be unconstrained")
	}
	if f.got.System == "" {
		t.Error("no system prompt was sent")
	}
}

func TestParsesDiagnosis(t *testing.T) {
	f := &fakeClient{reply: goodReply}
	d, err := Run(context.Background(), f, realExtract(t), Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if d.SchemaVersion != schema.Version {
		t.Errorf("SchemaVersion = %d", d.SchemaVersion)
	}
	if d.Target == "" || d.Diff == "" || d.Confidence != "medium" {
		t.Errorf("diagnosis not populated: %+v", d)
	}
	if _, err := json.Marshal(d); err != nil {
		t.Errorf("diagnosis is not serializable: %v", err)
	}
}

func TestEmptyDiffIsRejected(t *testing.T) {
	f := &fakeClient{reply: `{"target":"t","cause":"c","change":"ch","diff":"","confidence":"low","risks":[]}`}
	_, err := Run(context.Background(), f, realExtract(t), Options{})
	if err == nil || !strings.Contains(err.Error(), "empty diff") {
		t.Errorf("want empty-diff error, got %v", err)
	}
}

func TestNonJSONResponseIsAnError(t *testing.T) {
	f := &fakeClient{reply: "Sure! Here's what I'd do..."}
	if _, err := Run(context.Background(), f, realExtract(t), Options{}); err == nil {
		t.Error("prose response was accepted as a diagnosis")
	}
}

func TestNoHotspotsIsAnError(t *testing.T) {
	if _, err := Run(context.Background(), &fakeClient{}, &schema.ExtractResult{}, Options{}); err == nil {
		t.Error("want error for empty extract result")
	}
}

func TestAPIErrorIsWrapped(t *testing.T) {
	f := &fakeClient{err: errors.New("boom")}
	_, err := Run(context.Background(), f, realExtract(t), Options{})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("want wrapped API error, got %v", err)
	}
}
