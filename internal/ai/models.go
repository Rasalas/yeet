package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		return fetchACPModels(ctx, rp)
	case config.ProtocolPi:
		return fetchPiModels(ctx, rp)
	default:
		return fetchOpenAICompatible(ctx, rp)
	}
}

// fetchACPModels creates a no-prompt session and reads the model choices that
// the agent advertises. ACP agents that do not expose model state fall back to
// KnownModels in the TUI.
func fetchACPModels(ctx context.Context, rp config.ResolvedProvider) ([]string, error) {
	rp = resolveACPProvider(rp)
	if rp.Command == "" {
		return nil, fmt.Errorf("%s ACP command is not configured", rp.Name)
	}

	// Deliberately no model/reasoning overrides: discovery must see the
	// adapter's vanilla configuration.
	provider := &ACPProvider{Name: rp.Name, Command: rp.Command, Args: rp.Args}

	runCtx, cancel := context.WithTimeout(ctx, modelDiscoveryTimeout)
	defer cancel()

	conn, session, err := openACPSession(runCtx, provider, "Model discovery only. Do not generate text.")
	if err != nil {
		return nil, err
	}
	defer conn.close()

	models, err := parseACPModels(session)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("%s ACP agent did not advertise any models", rp.Name)
	}
	return models, nil
}

// parseACPModels extracts advertised models from a freshly created ACP
// session: config options named "model" first, then availableModels.
func parseACPModels(session acpSession) ([]string, error) {
	var models []string
	for _, option := range session.ConfigOptions {
		if option.ID == "model" {
			models = appendUnique(models, acpOptionValues(option.Options)...)
		}
	}
	if len(models) > 0 {
		return models, nil
	}

	for _, item := range session.Models.Available {
		models = appendUnique(models, item.ModelID)
	}
	models = appendUnique(models, session.Models.Current)
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
