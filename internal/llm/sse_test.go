package llm

import (
	"bytes"
	"strings"
	"testing"
)

func TestScanEvents(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantPayloads []string
		wantErr      bool
	}{
		{
			name: "normal events",
			input: `data: {"type":"start"}
data: {"type":"message"}
data: [DONE]`,
			wantPayloads: []string{
				`{"type":"start"}`,
				`{"type":"message"}`,
			},
		},
		{
			name: "blank lines are skipped",
			input: `data: first

data: second`,
			wantPayloads: []string{"first", "second"},
		},
		{
			name: "lines without data: prefix are skipped",
			input: `data: first
other: ignored
data: second`,
			wantPayloads: []string{"first", "second"},
		},
		{
			name: "[DONE] terminates scanning",
			input: `data: first
data: [DONE]
data: never_seen`,
			wantPayloads: []string{"first"},
		},
		{
			name: "optional space after colon is trimmed",
			input: `data:{"type":"no_space"}
data: {"type":"with_space"}`,
			wantPayloads: []string{
				`{"type":"no_space"}`,
				`{"type":"with_space"}`,
			},
		},
		{
			name:  "large payload exceeding default buffer",
			input: "data: " + strings.Repeat("x", 100000),
			wantPayloads: []string{
				strings.Repeat("x", 100000),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payloads []string
			err := ScanEvents(bytes.NewReader([]byte(tt.input)), func(data []byte) error {
				payloads = append(payloads, string(data))
				return nil
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("ScanEvents error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(payloads) != len(tt.wantPayloads) {
				t.Errorf("got %d payloads, want %d", len(payloads), len(tt.wantPayloads))
				return
			}

			for i, payload := range payloads {
				if payload != tt.wantPayloads[i] {
					t.Errorf("payload %d = %q, want %q", i, payload, tt.wantPayloads[i])
				}
			}
		})
	}
}
