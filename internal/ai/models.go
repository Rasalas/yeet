package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
)

var modelsClient = &http.Client{Timeout: 5 * time.Second}

const modelDiscoveryTimeout = 15 * time.Second

// FetchModels queries the provider or local agent for available models.
// API model IDs are sorted; native agents preserve their preferred order.
func FetchModels(ctx context.Context, provider string, cfg config.Config) ([]string, error) {
	rp, ok := cfg.ResolveProviderFull(provider)
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	switch rp.Protocol {
	case config.ProtocolAnthropic:
		return fetchAnthropic(ctx, rp)
	case config.ProtocolOllama:
		return fetchOllama(ctx, rp)
	case config.ProtocolACP:
		if provider == "codex" {
			return fetchCodexModels(ctx)
		}
		return fetchACPModels(ctx, rp)
	default:
		return fetchOpenAICompatible(ctx, rp)
	}
}

// fetchCodexModels asks the installed Codex CLI for the models visible to the
// current account. The app-server catalog is the same source used by Codex's
// own model picker and preserves its preferred ordering.
func fetchCodexModels(ctx context.Context) ([]string, error) {
	command, err := exec.LookPath("codex")
	if err != nil {
		return nil, fmt.Errorf("codex CLI is not installed: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(ctx, modelDiscoveryTimeout)
	defer cancel()
	conn, err := startACP(runCtx, command, []string{"app-server"}, cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to start Codex app server: %w", err)
	}
	defer conn.close()

	if _, err := conn.call(1, "initialize", map[string]any{
		"clientInfo": map[string]string{
			"name":    "yeet",
			"title":   "yeet",
			"version": "0.0.0",
		},
	}, nil); err != nil {
		return nil, err
	}

	var models []string
	var cursor string
	for id := int64(2); ; id++ {
		params := map[string]any{"includeHidden": false, "limit": 100}
		if cursor != "" {
			params["cursor"] = cursor
		}
		result, err := conn.call(id, "model/list", params, nil)
		if err != nil {
			return nil, err
		}

		page, next, err := parseCodexModelList(result)
		if err != nil {
			return nil, err
		}
		models = appendUnique(models, page...)
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("Codex returned no available models")
	}
	return models, nil
}

func parseCodexModelList(result json.RawMessage) ([]string, string, error) {
	var response struct {
		Data []struct {
			ID    string `json:"id"`
			Model string `json:"model"`
		} `json:"data"`
		NextCursor string `json:"nextCursor"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, "", fmt.Errorf("failed to parse Codex model list: %w", err)
	}

	models := make([]string, 0, len(response.Data))
	for _, item := range response.Data {
		name := item.Model
		if name == "" {
			name = item.ID
		}
		models = appendUnique(models, name)
	}
	return models, response.NextCursor, nil
}

// fetchACPModels creates a no-prompt session and reads the model choices that
// the agent advertises. ACP agents that do not expose model state fall back to
// KnownModels in the TUI.
func fetchACPModels(ctx context.Context, rp config.ResolvedProvider) ([]string, error) {
	rp = resolveACPProvider(rp)
	if rp.Command == "" {
		return nil, fmt.Errorf("%s ACP command is not configured", rp.Name)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(ctx, modelDiscoveryTimeout)
	defer cancel()
	provider := &ACPProvider{Name: rp.Name, Command: rp.Command, Args: rp.Args}
	conn, err := startACP(runCtx, provider.Command, provider.commandArgs(), cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to start %s ACP agent: %w", rp.Name, err)
	}
	defer conn.close()

	if _, err := conn.call(1, "initialize", map[string]any{
		"protocolVersion": acpProtocolVersion,
		"clientCapabilities": map[string]any{
			"fs":       map[string]bool{"readTextFile": false, "writeTextFile": false},
			"terminal": false,
		},
		"clientInfo": map[string]string{
			"name":    "yeet",
			"title":   "yeet",
			"version": "0.0.0",
		},
	}, nil); err != nil {
		return nil, err
	}

	result, err := conn.call(2, "session/new", map[string]any{
		"cwd":        cwd,
		"mcpServers": []any{},
		"_meta":      acpSessionMeta("Model discovery only. Do not generate text.", ""),
	}, nil)
	if err != nil {
		return nil, err
	}
	models, err := parseACPModels(result)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("%s ACP agent did not advertise any models", rp.Name)
	}
	return models, nil
}

func parseACPModels(result json.RawMessage) ([]string, error) {
	var response struct {
		Models struct {
			Available []struct {
				ModelID string `json:"modelId"`
			} `json:"availableModels"`
			Current string `json:"currentModelId"`
		} `json:"models"`
		ConfigOptions []struct {
			ID      string          `json:"id"`
			Options json.RawMessage `json:"options"`
		} `json:"configOptions"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse ACP model list: %w", err)
	}

	var models []string
	for _, item := range response.Models.Available {
		models = appendUnique(models, item.ModelID)
	}
	models = appendUnique(models, response.Models.Current)
	if len(models) > 0 {
		return models, nil
	}

	for _, option := range response.ConfigOptions {
		if option.ID == "model" {
			models = appendUnique(models, acpOptionValues(option.Options)...)
		}
	}
	return models, nil
}

func acpOptionValues(raw json.RawMessage) []string {
	var options []struct {
		Value   string          `json:"value"`
		Options json.RawMessage `json:"options"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &options) != nil {
		return nil
	}

	var values []string
	for _, option := range options {
		values = appendUnique(values, option.Value)
		values = appendUnique(values, acpOptionValues(option.Options)...)
	}
	return values
}

func appendUnique(values []string, candidates ...string) []string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		found := false
		for _, value := range values {
			if value == candidate {
				found = true
				break
			}
		}
		if !found {
			values = append(values, candidate)
		}
	}
	return values
}

func fetchAnthropic(ctx context.Context, rp config.ResolvedProvider) ([]string, error) {
	key, err := keyring.GetWithEnv(rp.Name, rp.Env)
	if err != nil {
		return nil, fmt.Errorf("no API key for %s", rp.Name)
	}

	url := strings.TrimRight(rp.URL, "/") + "/models?limit=100"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", anthropicVersion)

	return doOpenAIModelList(req)
}

func fetchOllama(ctx context.Context, rp config.ResolvedProvider) ([]string, error) {
	url := strings.TrimRight(rp.URL, "/") + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := modelsClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Models {
		name := m.Name
		name = strings.TrimSuffix(name, ":latest")
		models = append(models, name)
	}
	sort.Strings(models)
	return models, nil
}

func fetchOpenAICompatible(ctx context.Context, rp config.ResolvedProvider) ([]string, error) {
	key, err := keyring.GetWithEnv(rp.Name, rp.Env)
	if err != nil {
		return nil, fmt.Errorf("no API key for %s", rp.Name)
	}

	url := strings.TrimRight(rp.URL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)

	return doOpenAIModelList(req)
}

// doOpenAIModelList executes a request and parses the standard {"data": [{"id": "..."}]} response.
func doOpenAIModelList(req *http.Request) ([]string, error) {
	resp, err := modelsClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	sort.Strings(models)
	return models, nil
}
