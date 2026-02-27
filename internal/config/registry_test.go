package config

import "testing"

func TestRegistryCompleteness(t *testing.T) {
	// Every KnownModels key should have a Registry entry.
	for provider := range KnownModels {
		if _, ok := Registry[provider]; !ok {
			t.Errorf("KnownModels has %q but Registry does not", provider)
		}
	}
}

func TestRegistryProtocols(t *testing.T) {
	for name, entry := range Registry {
		switch entry.Protocol {
		case ProtocolAnthropic, ProtocolOpenAI, ProtocolOllama:
			// valid
		default:
			t.Errorf("Registry[%q] has unknown protocol %q", name, entry.Protocol)
		}
	}
}

func TestRegistryDefaults(t *testing.T) {
	for name, entry := range Registry {
		if entry.DefaultModel == "" {
			t.Errorf("Registry[%q] has empty DefaultModel", name)
		}
		if entry.DefaultURL == "" {
			t.Errorf("Registry[%q] has empty DefaultURL", name)
		}
		if entry.NeedsAuth && entry.DefaultEnv == "" {
			t.Errorf("Registry[%q] NeedsAuth but has no DefaultEnv", name)
		}
	}
}
