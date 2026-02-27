package keyring

import (
	"os"

	gokeyring "github.com/zalando/go-keyring"
)

const serviceName = "yeet"

var envKeys = map[string]string{
	"anthropic": "ANTHROPIC_API_KEY",
	"openai":    "OPENAI_API_KEY",
}

func Set(provider, apiKey string) error {
	return gokeyring.Set(serviceName, provider, apiKey)
}

func Get(provider string) (string, error) {
	key, err := gokeyring.Get(serviceName, provider)
	if err == nil && key != "" {
		return key, nil
	}
	if envVar, ok := envKeys[provider]; ok {
		if envKey := os.Getenv(envVar); envKey != "" {
			return envKey, nil
		}
	}
	return "", err
}

func Delete(provider string) error {
	return gokeyring.Delete(serviceName, provider)
}

func Status() map[string]bool {
	providers := []string{"anthropic", "openai", "ollama"}
	status := make(map[string]bool, len(providers))
	for _, p := range providers {
		key, err := Get(p)
		status[p] = err == nil && key != ""
	}
	return status
}
