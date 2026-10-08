package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathUsesXDGConfigHome(t *testing.T) {
	t.Setenv("FRIENDLY_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	got, err := Path("keys.toml")
	if err != nil || got != filepath.Join("/xdg", "friendly", "keys.toml") {
		t.Errorf("Path() = %q, %v, want /xdg/friendly/keys.toml", got, err)
	}
}

func TestPathFallsBackToDotConfig(t *testing.T) {
	t.Setenv("FRIENDLY_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/me")
	got, err := Path("theme.toml")
	if err != nil || got != filepath.Join("/home/me", ".config", "friendly", "theme.toml") {
		t.Errorf("Path() = %q, %v, want /home/me/.config/friendly/theme.toml", got, err)
	}
}

func TestPathUsesFriendlyConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FRIENDLY_CONFIG_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	got, err := Path("keys.toml")
	if err != nil || got != filepath.Join(dir, "keys.toml") {
		t.Errorf("Path() = %q, %v, want keys.toml in %s", got, err, dir)
	}
}

func TestPathRejectsFriendlyConfigDirThatIsNoFolder(t *testing.T) {
	file := filepath.Join(t.TempDir(), "keys.toml")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	for dir, want := range map[string]string{
		filepath.Join(t.TempDir(), "missing"): "no such file or directory",
		file:                                  "is not a folder",
	} {
		t.Setenv("FRIENDLY_CONFIG_DIR", dir)
		if _, err := Path("keys.toml"); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Path() with %s = %v, want an error with %q", dir, err, want)
		}
	}
}
