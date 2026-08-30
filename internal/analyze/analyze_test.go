package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/joaolaureano/profadvisor/internal/extract"
	"github.com/joaolaureano/profadvisor/internal/fixture"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

// fakeClient records the request and returns a canned response, so the prompt
// and the parsing can be tested without an API key or a network call.
type fakeClient struct {
	got   anthropic.MessageNewParams
	reply string
	err   error
}

func (f *fakeClient) NewMessage(_ context.Context, p anthropic.MessageNewParams) (*anthropic.Message, error) {
	f.got = p
	if f.err != nil {
		return nil, f.err
	}
	// Built by decoding an API-shaped payload rather than by filling the
	// struct in: ContentBlockUnion.AsAny re-unmarshals from the raw JSON it
	// was decoded from, so a hand-assembled Message would silently present
	// as having no content — exactly the bug this fake exists to catch.
	payload, err := json.Marshal(map[string]any{
		"id": "msg_test", "type": "message", "role": "assistant",
		"model": "claude-opus-5", "stop_reason": "end_turn",
		"content": []any{map[string]any{"type": "text", "text": f.reply}},
		"usage":   map[string]any{"input_tokens": 1, "output_tokens": 1},
	})
	if err != nil {
		return nil, err
	}
	var msg anthropic.Message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
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

	var prompt string
	for _, blk := range f.got.Messages[0].Content {
		if blk.OfText != nil {
			prompt += blk.OfText.Text
		}
	}

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
	// wanted, even — in the "where the filtered time went" block above it,
	// because that is what explains the cost. What must never happen is a
	// runtime frame appearing as something to edit.
	_, hotspots, found := strings.Cut(prompt, "# Hotspots, hottest first")
	if !found {
		t.Fatal("prompt has no hotspot section")
	}
	if strings.Contains(hotspots, "runtime.") {
		t.Error("a filtered runtime frame was ranked as a hotspot")
	}
	if !strings.Contains(prompt, "Where the filtered time went") {
		t.Error("the filtered time was reported as a total but not attributed")
	}
}

func TestRequestShape(t *testing.T) {
	r := realExtract(t)
	f := &fakeClient{reply: goodReply}
	if _, err := Run(context.Background(), f, r, Options{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := string(f.got.Model); got != DefaultModel {
		t.Errorf("model = %q, want %q", got, DefaultModel)
	}
	if f.got.Thinking.OfAdaptive == nil {
		t.Error("adaptive thinking was not requested")
	}
	if f.got.OutputConfig.Format.Schema == nil {
		t.Error("no output schema was sent; the response shape would be unconstrained")
	}
	if len(f.got.System) == 0 || f.got.System[0].CacheControl.Type == "" {
		t.Error("system prompt is not cached; it is identical across calls in a run")
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
