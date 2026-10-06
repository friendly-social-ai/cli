package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors follow Friendly web theme, tuned for text contrast on light and dark terminals.
var (
	ColorPrimary = lipgloss.AdaptiveColor{Light: "#0060D0", Dark: "#6E8BFF"}
	ColorMuted   = lipgloss.AdaptiveColor{Light: "#646464", Dark: "#B4B4B4"}
	ColorDanger  = lipgloss.AdaptiveColor{Light: "#C4391D", Dark: "#F0715A"}
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#18794E", Dark: "#3DD68C"}
	ColorBorder  = lipgloss.AdaptiveColor{Light: "#C8C8C8", Dark: "#4E4E4E"}

	// ColorOnAccent is text color on top of ColorPrimary or ColorSuccess backgrounds.
	ColorOnAccent = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#111111"}
)

var (
	MutedStyle  = lipgloss.NewStyle().Foreground(ColorMuted)
	AccentStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	DangerStyle = lipgloss.NewStyle().Foreground(ColorDanger)
	BoldStyle   = lipgloss.NewStyle().Bold(true)
)

// Fields renders "key: value" lines in muted keys, skipping pairs with empty value.
func Fields(pairs ...string) string {
	var lines []string
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] != "" {
			lines = append(lines, MutedStyle.Render(pairs[i]+": ")+pairs[i+1])
		}
	}

	return strings.Join(lines, "\n")
}
