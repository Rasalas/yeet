package keyring

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/rasalas/yeet/internal/xdg"
	gokeyring "github.com/zalando/go-keyring"
)

const serviceName = "yeet"

// KeySource indicates where a key was found in the lookup chain.
type KeySource string

const (
	SourceKeyring  KeySource = "keyring"
	SourceEnv      KeySource = "env"
	SourceOpenCode KeySource = "opencode"
	SourceNone     KeySource = ""
)


// Set stores a key in the OS keyring.
func Set(provider, apiKey string) error {
	return gokeyring.Set(serviceName, provider, apiKey)
}

// Resolve finds a key for the provider using the lookup chain:
//  1. OS Keyring
//  2. Env var (customEnv overrides the default mapping)
//  3. OpenCode auth.json (only type:"api" keys)
func Resolve(provider, customEnv string) (string, KeySource) {
	// 1. Keyring
	key, err := gokeyring.Get(serviceName, provider)
	if err == nil && key != "" {
		return key, SourceKeyring
	}

	// 2. Env var
	if customEnv != "" {
		if envKey := os.Getenv(customEnv); envKey != "" {
			return envKey, SourceEnv
		}
	}

	// 3. OpenCode auth.json
	if ocKey := readOpenCodeKey(provider); ocKey != "" {
		return ocKey, SourceOpenCode
	}

	return "", SourceNone
}

// GetWithEnv retrieves a key using a custom env var name.
func GetWithEnv(provider, customEnv string) (string, error) {
	key, source := Resolve(provider, customEnv)
	if source == SourceNone {
		return "", gokeyring.ErrNotFound
	}
	return key, nil
}

// Delete removes a key from the OS keyring.
func Delete(provider string) error {
	return gokeyring.Delete(serviceName, provider)
}

// KeyInfo holds the availability and source of a key.
type KeyInfo struct {
	Found  bool
	Source KeySource
}

// Status returns key availability and source for the given providers.
// customEnvs maps provider names to custom env var names (for custom providers).
func Status(providers []string, customEnvs map[string]string) map[string]KeyInfo {
	status := make(map[string]KeyInfo, len(providers))
	for _, p := range providers {
		key, source := Resolve(p, customEnvs[p])
		status[p] = KeyInfo{
			Found:  key != "",
			Source: source,
		}
	}
	return status
}

// openCodeAuth represents an entry in OpenCode's auth.json.
type openCodeAuth struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

func openCodeAuthPath() string {
	dir, err := xdg.DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "opencode", "auth.json")
}

func loadOpenCodeAuth() map[string]openCodeAuth {
	path := openCodeAuthPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var auth map[string]openCodeAuth
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil
	}
	return auth
}

// readOpenCodeKey reads a key from ~/.local/share/opencode/auth.json.
// Only returns keys with type:"api", ignoring oauth tokens.
func readOpenCodeKey(provider string) string {
	auth := loadOpenCodeAuth()
	entry, ok := auth[provider]
	if !ok {
		return ""
	}
	if entry.Type != "api" {
		return ""
	}
	return entry.Key
}

// OpenCodeProviders returns provider names that have type:"api" keys
// in OpenCode's auth.json.
func OpenCodeProviders() []string {
	auth := loadOpenCodeAuth()
	var providers []string
	for name, entry := range auth {
		if entry.Type == "api" && entry.Key != "" {
			providers = append(providers, name)
		}
	}
	return providers
}
