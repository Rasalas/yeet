package ai

import (
	"strings"
	"testing"
)

func TestParseSSE(t *testing.T) {
	t.Run("basic events", func(t *testing.T) {
		input := "event: message\ndata: hello\n\nevent: message\ndata: world\n\n"
		var events []struct{ typ, data string }

		err := parseSSE(strings.NewReader(input), func(eventType, data string) {
			events = append(events, struct{ typ, data string }{eventType, data})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 2 {
			t.Fatalf("got %d events, want 2", len(events))
		}
		if events[0].typ != "message" || events[0].data != "hello" {
			t.Errorf("event 0 = %+v", events[0])
		}
		if events[1].typ != "message" || events[1].data != "world" {
			t.Errorf("event 1 = %+v", events[1])
		}
	})

	t.Run("data only no event type", func(t *testing.T) {
		input := "data: {\"text\":\"hi\"}\n\n"
		var events []struct{ typ, data string }

		err := parseSSE(strings.NewReader(input), func(eventType, data string) {
			events = append(events, struct{ typ, data string }{eventType, data})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 1 {
			t.Fatalf("got %d events, want 1", len(events))
		}
		if events[0].typ != "" {
			t.Errorf("expected empty event type, got %q", events[0].typ)
		}
		if events[0].data != `{"text":"hi"}` {
			t.Errorf("data = %q", events[0].data)
		}
	})

	t.Run("DONE sentinel", func(t *testing.T) {
		input := "data: {\"content\":\"tok\"}\n\ndata: [DONE]\n\n"
		var datas []string

		err := parseSSE(strings.NewReader(input), func(eventType, data string) {
			datas = append(datas, data)
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(datas) != 2 {
			t.Fatalf("got %d events, want 2", len(datas))
		}
		if datas[1] != "[DONE]" {
			t.Errorf("expected [DONE], got %q", datas[1])
		}
	})

	t.Run("trailing event without blank line", func(t *testing.T) {
		input := "data: trailing"
		var datas []string

		err := parseSSE(strings.NewReader(input), func(eventType, data string) {
			datas = append(datas, data)
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(datas) != 1 {
			t.Fatalf("got %d events, want 1", len(datas))
		}
		if datas[0] != "trailing" {
			t.Errorf("data = %q", datas[0])
		}
	})

	t.Run("empty input", func(t *testing.T) {
		var count int
		err := parseSSE(strings.NewReader(""), func(eventType, data string) {
			count++
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("got %d events for empty input", count)
		}
	})

	t.Run("blank lines only", func(t *testing.T) {
		var count int
		err := parseSSE(strings.NewReader("\n\n\n"), func(eventType, data string) {
			count++
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("got %d events for blank lines", count)
		}
	})

	t.Run("anthropic style events", func(t *testing.T) {
		input := `event: message_start
data: {"message":{"usage":{"input_tokens":100}}}

event: content_block_delta
data: {"delta":{"text":"fix"}}

event: content_block_delta
data: {"delta":{"text":": typo"}}

event: message_delta
data: {"usage":{"output_tokens":5}}

`
		var events []struct{ typ, data string }
		err := parseSSE(strings.NewReader(input), func(eventType, data string) {
			events = append(events, struct{ typ, data string }{eventType, data})
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 4 {
			t.Fatalf("got %d events, want 4", len(events))
		}
		if events[0].typ != "message_start" {
			t.Errorf("event 0 type = %q", events[0].typ)
		}
		if events[1].typ != "content_block_delta" {
			t.Errorf("event 1 type = %q", events[1].typ)
		}
		if events[3].typ != "message_delta" {
			t.Errorf("event 3 type = %q", events[3].typ)
		}
	})
}
