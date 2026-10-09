package config

import (
	"log/slog"
	"path/filepath"
	"strconv"

	"github.com/editorconfig/editorconfig-core-go/v2"
)

// defaultIndent is the indent without the setting and without EditorConfig.
const defaultIndent = 4

// Indentation returns how many spaces the composer indents by. The indent setting comes first. Without it, Indentation
// asks EditorConfig about a markdown file in home, so ~/.editorconfig and the files above it apply. indent_size = tab
// gives tab_width. Without an EditorConfig indent it returns 4. Indentation logs and skips a broken EditorConfig or a
// width out of range, since other editors use the file too.
func (s Settings) Indentation(home string) int {
	if s.Indent != 0 {
		return s.Indent
	}

	if home == "" {
		return defaultIndent
	}

	definition, err := editorconfig.GetDefinitionForFilename(filepath.Join(home, "post.md"))
	if err != nil {
		slog.Warn("editorconfig", "err", err)
		return defaultIndent
	}

	indent, err := strconv.Atoi(definition.IndentSize)
	if err != nil {
		indent = definition.TabWidth
	}

	switch {
	case indent == 0:
		return defaultIndent
	case indent < 0 || indent > maxIndent:
		slog.Warn("editorconfig", "indent_size", indent, "max", maxIndent)
		return defaultIndent
	}

	slog.Info("editorconfig", "indent", indent)
	return indent
}
