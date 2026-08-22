package xdg

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConfigDir_Default(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	dir, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(".config", "yeet")
	if runtime.GOOS == "windows" {
		want = "yeet" // %AppData%\yeet
	}
	if !strings.HasSuffix(dir, want) {
		t.Fatalf("expected suffix %s, got %s", want, dir)
	}
}

func TestConfigDir_XDG(t *testing.T) {
	xdgRoot := filepath.Join(string(filepath.Separator), "tmp", "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdgRoot)
	dir, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(xdgRoot, "yeet") {
		t.Fatalf("expected %s, got %s", filepath.Join(xdgRoot, "yeet"), dir)
	}
}

func TestDataDir_Default(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	dir, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		// %LocalAppData% itself; base name varies by user profile.
		if dir == "" {
			t.Fatal("expected non-empty data dir")
		}
		return
	}
	want := filepath.Join(".local", "share")
	if !strings.HasSuffix(dir, want) {
		t.Fatalf("expected suffix %s, got %s", want, dir)
	}
}

func TestDataDir_XDG(t *testing.T) {
	xdgRoot := filepath.Join(string(filepath.Separator), "tmp", "xdg")
	t.Setenv("XDG_DATA_HOME", xdgRoot)
	dir, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != xdgRoot {
		t.Fatalf("expected %s, got %s", xdgRoot, dir)
	}
}
