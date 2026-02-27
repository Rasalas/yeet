package xdg

import (
	"os"
	"path/filepath"
)

// ConfigDir returns $XDG_CONFIG_HOME/yeet, defaulting to ~/.config/yeet.
func ConfigDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "yeet"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "yeet"), nil
}

// DataDir returns $XDG_DATA_HOME, defaulting to ~/.local/share.
func DataDir() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}
