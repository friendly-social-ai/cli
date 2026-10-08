package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Settings are the general settings of config.toml.
type Settings struct {
	// Images is how posts show images. auto draws them with terminal graphics when the terminal supports them and
	// with colored blocks otherwise. graphics and blocks skip the detection, and off shows no images.
	Images string
	// Refresh is how often the app checks for new posts and activity. Zero turns the checks off.
	Refresh time.Duration
}

// Defaults are the settings without a config file.
var Defaults = Settings{Images: "auto", Refresh: time.Minute}

// minRefresh is the shortest refresh, so the checks don't flood the server.
const minRefresh = 30 * time.Second

// Load reads the settings at path, keeping the default of each setting the file leaves out. A missing file gives the
// defaults. A file with mistakes gives an error that lists every mistake.
func Load(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		slog.Info("settings", "path", path, "found", false)
		return Defaults, nil
	}

	if err != nil {
		return Defaults, fmt.Errorf("config: failed to read %s: %w", path, err)
	}

	var file struct {
		Images  string `toml:"images"`
		Refresh string `toml:"refresh"`
	}

	meta, err := toml.Decode(string(data), &file)
	if err != nil {
		return Defaults, fmt.Errorf("config: %s: %w", path, err)
	}

	var problems []string
	for _, k := range meta.Undecoded() {
		problems = append(problems, "unknown setting "+k.String())
	}

	settings := Defaults
	if meta.IsDefined("images") {
		if !slices.Contains([]string{"auto", "graphics", "blocks", "off"}, file.Images) {
			problems = append(problems, fmt.Sprintf("images: %q is not auto, graphics, blocks or off", file.Images))
		}

		settings.Images = file.Images
	}

	if meta.IsDefined("refresh") {
		refresh, err := time.ParseDuration(file.Refresh)
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("refresh: %q is not a duration, like 30s or 5m", file.Refresh))
		case refresh != 0 && refresh < minRefresh:
			problems = append(problems, fmt.Sprintf("refresh: %q is shorter than %s, use 0 to turn it off",
				file.Refresh, minRefresh))
		}

		settings.Refresh = refresh
	}

	if problems != nil {
		return Defaults, fmt.Errorf("config: %s:\n  %s", path, strings.Join(problems, "\n  "))
	}

	slog.Info("settings", "path", path, "found", true, "images", settings.Images, "refresh", settings.Refresh)
	return settings, nil
}
