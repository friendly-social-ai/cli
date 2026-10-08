package ui

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// themeFile is the user theme. Mode picks the colors of a dark or a light background, or asks the terminal when it is
// auto or empty. Dark and Light replace default colors by name.
type themeFile struct {
	Mode  string            `toml:"mode"`
	Dark  map[string]string `toml:"dark"`
	Light map[string]string `toml:"light"`
}

// hexColor matches a color written as #RGB or #RRGGBB.
var hexColor = regexp.MustCompile(`^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6})$`)

// LoadTheme reads the user theme at path and applies it. It calls dark to ask the terminal for its background only
// when the mode is auto. A missing file keeps the default colors. A file with mistakes changes nothing, and the error
// lists every mistake.
func LoadTheme(path string, dark func() bool) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		SetTheme(dark())
		return nil
	}

	if err != nil {
		return fmt.Errorf("theme: failed to read %s: %w", path, err)
	}

	var file themeFile
	meta, err := toml.Decode(string(data), &file)
	if err != nil {
		return fmt.Errorf("theme: %s: %w", path, err)
	}

	var problems []string
	for _, k := range meta.Undecoded() {
		problems = append(problems, "unknown setting "+k.String())
	}

	if !slices.Contains([]string{"", "auto", "dark", "light"}, file.Mode) {
		problems = append(problems, fmt.Sprintf("mode: %q is not auto, dark or light", file.Mode))
	}

	onLight, lightProblems := override(lightColors, "light", file.Light)
	onDark, darkProblems := override(darkColors, "dark", file.Dark)
	problems = append(append(problems, lightProblems...), darkProblems...)
	if problems != nil {
		return fmt.Errorf("theme: %s:\n  %s", path, strings.Join(problems, "\n  "))
	}

	switch {
	case file.Mode == "dark", file.Mode != "light" && dark():
		apply(true, onDark)
	default:
		apply(false, onLight)
	}

	return nil
}

// override returns c with the colors in set replaced by name, and the problems it found. Problems name a color by
// section and name, like dark.primary.
func override(c colors, section string, set map[string]string) (colors, []string) {
	slots := map[string]*string{
		"primary":   &c.primary,
		"muted":     &c.muted,
		"danger":    &c.danger,
		"border":    &c.border,
		"selection": &c.selection,
	}

	var problems []string
	for _, name := range slices.Sorted(maps.Keys(set)) {
		value := set[name]
		slot, ok := slots[name]
		switch {
		case !ok:
			problems = append(problems, "unknown color "+section+"."+name)
		case !validColor(value):
			problems = append(problems,
				fmt.Sprintf("%s.%s: %q is not a color, use #RRGGBB or 0 to 255", section, name, value))
		default:
			*slot = value
		}
	}

	return c, problems
}

// validColor reports whether value is a hex color or an ANSI color number.
func validColor(value string) bool {
	if hexColor.MatchString(value) {
		return true
	}

	n, err := strconv.Atoi(value)
	return err == nil && n >= 0 && n <= 255
}
