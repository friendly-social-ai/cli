package config

import (
	"path/filepath"
	"testing"
)

func TestPathUsesXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	got, err := Path("keys.toml")
	if err != nil || got != filepath.Join("/xdg", "friendly", "keys.toml") {
		t.Errorf("Path() = %q, %v, want /xdg/friendly/keys.toml", got, err)
	}
}

func TestPathFallsBackToDotConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/me")
	got, err := Path("theme.toml")
	if err != nil || got != filepath.Join("/home/me", ".config", "friendly", "theme.toml") {
		t.Errorf("Path() = %q, %v, want /home/me/.config/friendly/theme.toml", got, err)
	}
}
