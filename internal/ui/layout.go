package ui

import (
	"strings"
	"unicode"

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

// SplitConjuncts puts a word joiner before each letter that Unicode joins into the grapheme cluster of the letter
// before it, as in the conjunct क्ष. tmux and Ghostty draw such a letter in a cell of its own. A renderer that measures
// grapheme clusters then counts fewer cells than they draw, and moves the cursor to the wrong cells for the rest of
// the line. The word joiner ends the cluster and takes no cell. A zero width non-joiner also ends the cluster, but
// Ghostty then draws a Malayalam conjunct one cell wider.
func SplitConjuncts(s string) string {
	var b strings.Builder
	for len(s) > 0 {
		cluster, _ := ansi.FirstGraphemeCluster(s, ansi.GraphemeWidth)
		prev := rune(-1)
		for _, r := range cluster {
			if prev >= 0 && !unicode.IsLetter(prev) && unicode.IsLetter(r) {
				b.WriteRune('\u2060')
			}
			b.WriteRune(r)
			prev = r
		}
		s = s[len(cluster):]
	}

	return b.String()
}
