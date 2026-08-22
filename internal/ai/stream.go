package ai

import (
	"bufio"
	"io"
	"strings"
)

// StreamingProvider extends Provider with token-by-token streaming support.
type StreamingProvider interface {
	Provider
	GenerateCommitMessageStream(ctx CommitContext, onToken func(string)) (string, Usage, error)
}

// sseMaxLineSize caps the size of a single SSE line. The bufio.Scanner
// default (64 KiB) is too small for unusually large events.
const sseMaxLineSize = 1024 * 1024

// parseSSE reads Server-Sent Events from a reader and calls the handler for
// each event. It implements the subset of the SSE format used by AI
// providers: "event:" and "data:" fields, ":" comment lines, and blank-line
// dispatch. Multiple data: lines in one event are joined with "\n", and only
// a single leading space is stripped from field values, per spec.
// Returns any scanner error encountered during reading.
func parseSSE(r io.Reader, handler func(eventType, data string)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), sseMaxLineSize)

	var eventType string
	var data strings.Builder
	hasData := false

	dispatch := func() {
		if hasData {
			handler(eventType, data.String())
		}
		eventType = ""
		data.Reset()
		hasData = false
	}

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case line == "":
			// Blank line = end of event
			dispatch()
		case strings.HasPrefix(line, ":"):
			// Comment/keepalive line — ignored.
		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			value := strings.TrimPrefix(line, "data:")
			value = strings.TrimPrefix(value, " ") // exactly one leading space per spec
			if hasData {
				data.WriteByte('\n')
			}
			data.WriteString(value)
			hasData = true
		}
	}

	// Handle any trailing event without a final blank line
	dispatch()

	return scanner.Err()
}
