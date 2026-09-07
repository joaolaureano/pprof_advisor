package llm

import (
	"bufio"
	"bytes"
	"io"
)

// ScanEvents reads a text/event-stream body and calls fn with the payload of
// each `data:` line. It is exported so provider subpackages can share one
// framing implementation rather than duplicating the buffering and parsing logic.
//
// Behavior: uses bufio.Scanner with a 1 MB buffer (large SSE payloads can
// carry entire JSON objects). Skips blank lines and lines not starting with
// `data:`. Trims exactly one optional space after the colon. Stops cleanly on
// the payload `[DONE]`. Returns scanner.Err() if scanning fails.
func ScanEvents(r io.Reader, fn func(data []byte) error) error {
	scanner := bufio.NewScanner(r)
	// Enlarge the buffer to 1 MB so a single SSE data line can carry a large JSON.
	buf := make([]byte, 0, 64*1024) // initial
	scanner.Buffer(buf, 1024*1024)  // max 1 MB

	for scanner.Scan() {
		line := scanner.Bytes()

		// Skip blank lines.
		if len(line) == 0 {
			continue
		}

		// Check for data: prefix.
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}

		// Trim "data:" and exactly one optional space.
		payload := line[5:] // skip "data:"
		if len(payload) > 0 && payload[0] == ' ' {
			payload = payload[1:]
		}

		// Check for the [DONE] terminator.
		if bytes.Equal(payload, []byte("[DONE]")) {
			return nil
		}

		// Call the handler with the payload.
		if err := fn(payload); err != nil {
			return err
		}
	}

	return scanner.Err()
}
