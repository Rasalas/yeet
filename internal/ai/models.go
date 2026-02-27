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

// FetchModels queries the provider's API for available models.
// Returns a sorted list of model IDs, or an error if the request fails.
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
	default:
		return fetchOpenAICompatible(ctx, rp)
	}
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
