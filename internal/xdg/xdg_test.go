package xdg

import (
	"strings"
	"testing"
)

func TestConfigDir_Default(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	dir, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dir, ".config/yeet") {
		t.Fatalf("expected suffix .config/yeet, got %s", dir)
	}
}

func TestConfigDir_XDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	dir, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/tmp/xdg/yeet" {
		t.Fatalf("expected /tmp/xdg/yeet, got %s", dir)
	}
}

func TestDataDir_Default(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	dir, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dir, ".local/share") {
		t.Fatalf("expected suffix .local/share, got %s", dir)
	}
}

func TestDataDir_XDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	dir, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/tmp/xdg" {
		t.Fatalf("expected /tmp/xdg, got %s", dir)
	}
}
