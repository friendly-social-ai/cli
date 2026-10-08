package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// loadTheme writes content as a theme file and loads it with detect as the terminal background. The default dark
// theme comes back after the test.
func loadTheme(t *testing.T, content string, detect func() bool) error {
	t.Helper()
	t.Cleanup(func() { SetTheme(true) })
	path := filepath.Join(t.TempDir(), "theme.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return LoadTheme(path, detect)
}

func detected(dark bool) func() bool {
	return func() bool { return dark }
}

func TestLoadThemeMissingFileDetectsBackground(t *testing.T) {
	t.Cleanup(func() { SetTheme(true) })
	if err := LoadTheme(filepath.Join(t.TempDir(), "theme.toml"), detected(false)); err != nil {
		t.Fatalf("LoadTheme() = %v, want nil", err)
	}

	if palette.dark || palette.primary != lightColors.primary {
		t.Errorf("theme dark %v primary %q, want light %q", palette.dark, palette.primary, lightColors.primary)
	}
}

func TestLoadThemeModeSkipsDetection(t *testing.T) {
	detect := func() bool {
		t.Error("detect called with mode dark")
		return false
	}

	err := loadTheme(t, "mode = \"dark\"\n[dark]\nprimary = \"#CBA6F7\"\n[light]\nprimary = \"63\"\n", detect)
	if err != nil {
		t.Fatalf("LoadTheme() = %v, want nil", err)
	}

	if !palette.dark || palette.primary != "#CBA6F7" || palette.muted != darkColors.muted {
		t.Errorf("theme dark %v primary %q muted %q, want dark #CBA6F7 with default muted", palette.dark,
			palette.primary, palette.muted)
	}
}

func TestLoadThemeAutoUsesDetectedSection(t *testing.T) {
	if err := loadTheme(t, "[light]\nprimary = \"63\"\n", detected(false)); err != nil {
		t.Fatalf("LoadTheme() = %v, want nil", err)
	}

	if palette.dark || palette.primary != "63" {
		t.Errorf("theme dark %v primary %q, want light 63", palette.dark, palette.primary)
	}
}

func TestLoadThemeRejectsMistakes(t *testing.T) {
	err := loadTheme(t, `
mode = "dim"
accent = "#FFFFFF"

[dark]
primary = "red"
glow = "#FFFFFF"

[light]
muted = "256"
border = "#12"
`, detected(true))
	if err == nil {
		t.Fatal("LoadTheme() = nil, want an error")
	}

	for _, want := range []string{
		`mode: "dim" is not auto, dark or light`,
		"unknown setting accent",
		"unknown color dark.glow",
		`dark.primary: "red" is not a color`,
		`light.muted: "256" is not a color`,
		`light.border: "#12" is not a color`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("LoadTheme() = %v, want it to list %q", err, want)
		}
	}

	if palette.primary != darkColors.primary {
		t.Errorf("theme primary %q after mistakes, want default %q", palette.primary, darkColors.primary)
	}
}

// docs/theme.toml is the default theme users copy, so it must list every color with its default.
func TestDocsThemeListsDefaults(t *testing.T) {
	data, err := os.ReadFile("../../docs/theme.toml")
	if err != nil {
		t.Fatal(err)
	}

	var file themeFile
	if _, err := toml.Decode(string(data), &file); err != nil {
		t.Fatal(err)
	}

	for _, section := range []struct {
		name     string
		set      map[string]string
		defaults colors
	}{{"light", file.Light, lightColors}, {"dark", file.Dark, darkColors}} {
		got, problems := override(section.defaults, section.name, section.set)
		if problems != nil || got != section.defaults || len(section.set) != 5 {
			t.Errorf("docs/theme.toml [%s] = %v, want every color at its default %+v", section.name, section.set,
				section.defaults)
		}
	}

	if err := loadTheme(t, string(data), detected(true)); err != nil {
		t.Errorf("LoadTheme(docs/theme.toml) = %v, want nil", err)
	}
}
