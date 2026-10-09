package ui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSplitConjunctsMatchesTerminalWidth(t *testing.T) {
	// widths are what tmux 3.7 and Ghostty 1.3 draw in grapheme mode
	tests := []struct {
		name, text string
		want       int
	}{
		{"malayalam conjunct in a kaomoji", "Compile to wasm ദ്ദി(˵ •̀ ᴗ - ˵ ) ✧", 32},
		{"devanagari conjunct", "क्ष", 2},
		{"khmer subscript", "ក្ក", 2},
		{"combining accent", "•̀", 1},
		{"zwj emoji", "👨‍👩‍👧", 2},
		{"hangul jamo", "\u1100\u1161", 2},
		{"styled text", "\x1b[1mक्ष\x1b[m", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ansi.StringWidth(SplitConjuncts(tt.text)); got != tt.want {
				t.Errorf("width of SplitConjuncts(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestSplitConjunctsKeepsTextWithoutConjuncts(t *testing.T) {
	text := "plain, é, 👍🏽, " + Placeholder(1, 2, 1)
	if got := SplitConjuncts(text); got != text {
		t.Errorf("SplitConjuncts(%q) = %q, want it unchanged", text, got)
	}
}
