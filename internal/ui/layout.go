package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Clip fits view into width x height cells. It cuts long lines but never pads short ones, which leaves slack for
// emoji that terminals draw wider than measured. It adds empty lines to keep anything below in place.
func Clip(view string, width, height int) string {
	if height <= 0 {
		return ""
	}

	lines := strings.Split(view, "\n")
	lines = lines[:min(len(lines), height)]
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "")
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
