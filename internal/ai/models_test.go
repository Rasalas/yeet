package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/rasalas/yeet/internal/config"
)

func TestFetchModelsFromACP(t *testing.T) {
	cfg := config.Config{Custom: map[string]config.ProviderConfig{
		"fake-models": {
			Protocol: config.ProtocolACP,
			Command:  os.Args[0],
			Args:     []string{"-test.run=TestFakeACPModelAgent", "--", "acp-model-fake"},
		},
	}}

	got, err := FetchModels(context.Background(), "fake-models", cfg)
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	want := []string{"default", "claude-sonnet-5", "claude-fable-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("models = %v, want %v", got, want)
	}
}

func TestParseACPModelsFromNestedConfigOptions(t *testing.T) {
	result := json.RawMessage(`{
		"configOptions": [{
			"id": "model",
			"options": [
				{"value": "fast"},
				{"name": "Advanced", "options": [
					{"value": "smart"},
					{"value": "fast"}
				]}
			]
		}]
	}`)

	got, err := parseACPModels(result)
	if err != nil {
		t.Fatalf("parseACPModels: %v", err)
	}
	want := []string{"fast", "smart"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("models = %v, want %v", got, want)
	}
}

func TestParseCodexModelList(t *testing.T) {
	result := json.RawMessage(`{
		"data": [
			{"id": "display-id", "model": "gpt-5.5"},
			{"id": "gpt-5.4", "model": ""},
			{"id": "duplicate", "model": "gpt-5.5"}
		],
		"nextCursor": "next-page"
	}`)

	got, cursor, err := parseCodexModelList(result)
	if err != nil {
		t.Fatalf("parseCodexModelList: %v", err)
	}
	want := []string{"gpt-5.5", "gpt-5.4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("models = %v, want %v", got, want)
	}
	if cursor != "next-page" {
		t.Fatalf("cursor = %q, want next-page", cursor)
	}
}

func TestFakeACPModelAgent(t *testing.T) {
	if len(os.Args) == 0 || os.Args[len(os.Args)-1] != "acp-model-fake" {
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var msg acpMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			os.Exit(2)
		}
		switch msg.Method {
		case "initialize":
			writeFakeACP(enc, msg.ID, map[string]any{
				"protocolVersion": acpProtocolVersion,
				"agentCapabilities": map[string]any{
					"promptCapabilities": map[string]bool{},
				},
				"authMethods": []any{},
			})
		case "session/new":
			writeFakeACP(enc, msg.ID, map[string]any{
				"sessionId": "model-session",
				"models": map[string]any{
					"availableModels": []map[string]string{
						{"modelId": "default"},
						{"modelId": "claude-sonnet-5"},
					},
					"currentModelId": "claude-fable-5",
				},
			})
			os.Exit(0)
		}
	}
	os.Exit(3)
}
