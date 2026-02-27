package ai

import (
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
func FetchModels(provider string, cfg config.Config) ([]string, error) {
	switch provider {
	case "anthropic":
		return fetchAnthropic(cfg)
	case "ollama":
		return fetchOllama(cfg)
	default:
		return fetchOpenAICompatible(provider, cfg)
	}
}

func fetchAnthropic(cfg config.Config) ([]string, error) {
	key, err := keyring.Get("anthropic")
	if err != nil {
		return nil, fmt.Errorf("no API key for anthropic")
	}

	req, err := http.NewRequest("GET", "https://api.anthropic.com/v1/models?limit=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")

	return doOpenAIModelList(req)
}

func fetchOllama(cfg config.Config) ([]string, error) {
	url := cfg.Ollama.URL
	if url == "" {
		url = "http://localhost:11434"
	}
	url = strings.TrimRight(url, "/") + "/api/tags"

	req, err := http.NewRequest("GET", url, nil)
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

func fetchOpenAICompatible(provider string, cfg config.Config) ([]string, error) {
	var apiKey, baseURL string

	switch provider {
	case "openai":
		key, err := keyring.Get("openai")
		if err != nil {
			return nil, fmt.Errorf("no API key for openai")
		}
		apiKey = key
		baseURL = cfg.OpenAI.URL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
	default:
		pc, ok := cfg.ResolveProvider(provider)
		if !ok {
			return nil, fmt.Errorf("unknown provider: %s", provider)
		}
		key, err := keyring.GetWithEnv(provider, pc.Env)
		if err != nil {
			return nil, fmt.Errorf("no API key for %s", provider)
		}
		apiKey = key
		baseURL = pc.URL
	}

	url := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

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
