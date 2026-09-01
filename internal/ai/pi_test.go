package ai

import (
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rasalas/yeet/internal/config"
)

func TestPiProviderStreamsEphemeralToolFreeRun(t *testing.T) {
	provider := &PiProvider{
		Name:     "pi",
		Command:  os.Args[0],
		Args:     []string{"-test.run=TestFakePiAgent", "--", "pi-generate-fake"},
		Upstream: "anthropic",
		Model:    "claude-test",
	}

	var streamed strings.Builder
	message, usage, err := provider.GenerateCommitMessageStream(CommitContext{Diff: "diff --git a/a b/a"}, func(token string) {
		streamed.WriteString(token)
	})
	if err != nil {
		t.Fatalf("GenerateCommitMessageStream: %v", err)
	}
	if message != "fix: use Pi" {
		t.Fatalf("message = %q", message)
	}
	if streamed.String() != message {
		t.Fatalf("streamed = %q, want %q", streamed.String(), message)
	}
	if usage.Model != "pi/anthropic · claude-test" || usage.InputTokens != 42 || usage.OutputTokens != 4 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestFetchPiModels(t *testing.T) {
	rp := config.ResolvedProvider{
		Name:     "pi",
		Command:  os.Args[0],
		Args:     []string{"-test.run=TestFakePiAgent", "--", "pi-models-filter-fake"},
		Upstream: "anthropic",
		Protocol: config.ProtocolPi,
	}

	got, err := fetchPiModels(t.Context(), rp)
	if err != nil {
		t.Fatalf("fetchPiModels: %v", err)
	}
	want := []string{"claude-fast", "claude-smart"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("models = %v, want %v", got, want)
	}
}

func TestFetchPiUpstreams(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Custom = map[string]config.ProviderConfig{
		"pi": {
			Command: os.Args[0],
			Args:    []string{"-test.run=TestFakePiAgent", "--", "pi-models-all-fake"},
		},
	}

	got, err := FetchPiUpstreams(t.Context(), "pi", cfg)
	if err != nil {
		t.Fatalf("FetchPiUpstreams: %v", err)
	}
	want := []string{"openai-codex", "anthropic", "google"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("upstreams = %v, want %v", got, want)
	}
}

func TestFakePiAgent(t *testing.T) {
	if hasTestArg("pi-models-filter-fake") {
		if !hasArgPair(os.Args, "--list-models", "anthropic") {
			os.Exit(2)
		}
		writeFakePiModels()
		os.Exit(0)
	}
	if hasTestArg("pi-models-all-fake") {
		if !hasTestArg("--list-models") {
			os.Exit(2)
		}
		writeFakePiModels()
		os.Exit(0)
	}
	if !hasTestArg("pi-generate-fake") {
		return
	}

	requiredFlags := []string{"--no-session", "--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files"}
	for _, flag := range requiredFlags {
		if !hasTestArg(flag) {
			_, _ = io.WriteString(os.Stderr, "missing "+flag)
			os.Exit(2)
		}
	}
	for _, pair := range [][2]string{{"--mode", "json"}, {"--provider", "anthropic"}, {"--model", "claude-test"}, {"--thinking", "low"}} {
		if !hasArgPair(os.Args, pair[0], pair[1]) {
			_, _ = io.WriteString(os.Stderr, "missing "+pair[0]+" "+pair[1])
			os.Exit(2)
		}
	}
	prompt, _ := io.ReadAll(os.Stdin)
	if !strings.Contains(string(prompt), "diff --git a/a b/a") {
		_, _ = io.WriteString(os.Stderr, "missing prompt context")
		os.Exit(2)
	}

	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(map[string]any{
		"type":                  "message_update",
		"assistantMessageEvent": map[string]any{"type": "text_delta", "delta": "fix: "},
	})
	_ = enc.Encode(map[string]any{
		"type":                  "message_update",
		"assistantMessageEvent": map[string]any{"type": "text_delta", "delta": "use Pi"},
	})
	_ = enc.Encode(map[string]any{
		"type": "message_end",
		"message": map[string]any{
			"role": "assistant", "model": "claude-test", "stopReason": "stop",
			"usage": map[string]int{"input": 42, "output": 4},
		},
	})
	os.Exit(0)
}

func writeFakePiModels() {
	_, _ = io.WriteString(os.Stdout, "provider model context max-out thinking images\nopenai-codex gpt-fast 128K 32K yes no\nanthropic claude-fast 128K 32K yes yes\nanthropic claude-smart 272K 128K yes yes\ngoogle gemini-fast 128K 32K yes yes\n")
}

func hasTestArg(want string) bool {
	for _, arg := range os.Args {
		if arg == want {
			return true
		}
	}
	return false
}

func hasArgPair(args []string, key, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}
