package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

func loadSettings(t *testing.T, content string) (Settings, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return Load(path)
}

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil || got != Defaults {
		t.Errorf("Load() = %+v, %v, want defaults %+v", got, err, Defaults)
	}
}

func TestLoadKeepsDefaultsOfLeftOutSettings(t *testing.T) {
	got, err := loadSettings(t, "images = \"off\"\n")
	if err != nil || got.Images != "off" || got.Refresh != time.Minute {
		t.Errorf("Load() = %+v, %v, want images off and refresh 1m", got, err)
	}
}

func TestLoadTurnsRefreshOff(t *testing.T) {
	got, err := loadSettings(t, "refresh = \"0\"\n")
	if err != nil || got.Refresh != 0 {
		t.Errorf("Load() = %+v, %v, want refresh 0", got, err)
	}
}

func TestLoadRejectsMistakes(t *testing.T) {
	_, err := loadSettings(t, "images = \"sixel\"\nrefresh = \"5\"\ntheme = \"dark\"\n")
	if err == nil {
		t.Fatal("Load() = nil error, want an error")
	}

	for _, want := range []string{
		`images: "sixel" is not auto, graphics, blocks or off`,
		`refresh: "5" is not a duration`,
		"unknown setting theme",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Load() = %v, want it to list %q", err, want)
		}
	}
}

func TestLoadRejectsShortRefresh(t *testing.T) {
	if _, err := loadSettings(t, "refresh = \"5s\"\n"); err == nil || !strings.Contains(err.Error(), "shorter than 30s") {
		t.Errorf("Load() = %v, want an error about the shortest refresh", err)
	}
}

// docs/config.toml is the default config users copy, so it must give every setting its default.
func TestDocsConfigListsDefaults(t *testing.T) {
	data, err := os.ReadFile("../../docs/config.toml")
	if err != nil {
		t.Fatal(err)
	}

	var file map[string]any
	if err := toml.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"images", "refresh"} {
		if _, ok := file[name]; !ok {
			t.Errorf("docs/config.toml misses %s", name)
		}
	}

	if got, err := loadSettings(t, string(data)); err != nil || got != Defaults {
		t.Errorf("docs/config.toml gives %+v, %v, want the defaults %+v", got, err, Defaults)
	}
}
