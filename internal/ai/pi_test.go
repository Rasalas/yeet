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
		Name:    "pi",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestFakePiAgent", "--", "pi-generate-fake"},
		Model:   "gpt-test",
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
	if usage.Model != "pi · gpt-test" || usage.InputTokens != 42 || usage.OutputTokens != 4 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestFetchPiModels(t *testing.T) {
	rp := config.ResolvedProvider{
		Name:     "pi",
		Command:  os.Args[0],
		Args:     []string{"-test.run=TestFakePiAgent", "--", "pi-models-fake"},
		Protocol: config.ProtocolPi,
	}

	got, err := fetchPiModels(t.Context(), rp)
	if err != nil {
		t.Fatalf("fetchPiModels: %v", err)
	}
	want := []string{"gpt-fast", "gpt-smart"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("models = %v, want %v", got, want)
	}
}

func TestFakePiAgent(t *testing.T) {
	if hasTestArg("pi-models-fake") {
		if !hasArgPair(os.Args, "--list-models", "openai-codex") {
			os.Exit(2)
		}
		_, _ = io.WriteString(os.Stdout, "provider model context max-out thinking images\nopenai gpt-other 1K 1K no no\nopenai-codex gpt-fast 128K 32K yes no\nopenai-codex gpt-smart 272K 128K yes yes\n")
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
	for _, pair := range [][2]string{{"--mode", "json"}, {"--provider", "openai-codex"}, {"--model", "gpt-test"}, {"--thinking", "low"}} {
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
			"role": "assistant", "model": "gpt-test", "stopReason": "stop",
			"usage": map[string]int{"input": 42, "output": 4},
		},
	})
	os.Exit(0)
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
