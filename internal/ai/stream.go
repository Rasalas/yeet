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

// parseSSE reads Server-Sent Events from a reader and calls the handler for each event.
// It recognizes "event:" and "data:" fields and triggers the callback on blank lines.
func parseSSE(r io.Reader, handler func(eventType, data string)) {
	scanner := bufio.NewScanner(r)
	var eventType, data string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Blank line = end of event
			if data != "" {
				handler(eventType, data)
			}
			eventType = ""
			data = ""
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}

	// Handle any trailing event without a final blank line
	if data != "" {
		handler(eventType, data)
	}
}
