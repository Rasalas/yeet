package xdg

import (
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir returns $XDG_CONFIG_HOME/yeet, defaulting to ~/.config/yeet
// (%AppData%\yeet on Windows).
func ConfigDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "yeet"), nil
	}
	if runtime.GOOS == "windows" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "yeet"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "yeet"), nil
}

// DataDir returns $XDG_DATA_HOME, defaulting to ~/.local/share
// (%LocalAppData% on Windows).
func DataDir() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir, nil
	}
	if runtime.GOOS == "windows" {
		dir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}
