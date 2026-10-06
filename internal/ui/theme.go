package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// Colors follow the Friendly web theme, adjusted for text contrast on light and dark terminals. SetTheme picks them.
var (
	ColorPrimary color.Color
	ColorMuted   color.Color
	ColorDanger  color.Color
	ColorSuccess color.Color
	ColorBorder  color.Color

	// ColorOnAccent is text color on top of ColorPrimary or ColorSuccess backgrounds.
	ColorOnAccent color.Color
)

var (
	MutedStyle  lipgloss.Style
	AccentStyle lipgloss.Style
	DangerStyle lipgloss.Style
	BoldStyle   = lipgloss.NewStyle().Bold(true)
)

var (
	// inputStyle frames text inputs. The focused one gets a highlighted border.
	inputStyle        lipgloss.Style
	inputFocusedStyle lipgloss.Style

	// listMarker marks every line of the selected item. Unselected items get an indent of the same width.
	listMarker string
)

func init() {
	SetTheme(true)
}

// SetTheme picks colors for dark or light terminal background. Call it before building the UI, since rendered text
// keeps the colors it was rendered with.
func SetTheme(dark bool) {
	pick := lipgloss.LightDark(dark)
	ColorPrimary = pick(lipgloss.Color("#0060D0"), lipgloss.Color("#6E8BFF"))
	ColorMuted = pick(lipgloss.Color("#646464"), lipgloss.Color("#B4B4B4"))
	ColorDanger = pick(lipgloss.Color("#C4391D"), lipgloss.Color("#F0715A"))
	ColorSuccess = pick(lipgloss.Color("#18794E"), lipgloss.Color("#3DD68C"))
	ColorBorder = pick(lipgloss.Color("#C8C8C8"), lipgloss.Color("#4E4E4E"))
	ColorOnAccent = pick(lipgloss.Color("#FFFFFF"), lipgloss.Color("#111111"))

	MutedStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	AccentStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	DangerStyle = lipgloss.NewStyle().Foreground(ColorDanger)

	inputStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	inputFocusedStyle = inputStyle.BorderForeground(ColorPrimary)
	listMarker = lipgloss.NewStyle().Foreground(ColorPrimary).Render("▎ ")
}

// Fields renders "key: value" lines with muted keys. It skips pairs with an empty value.
func Fields(pairs ...string) string {
	var lines []string
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] != "" {
			lines = append(lines, MutedStyle.Render(pairs[i]+": ")+pairs[i+1])
		}
	}

	return strings.Join(lines, "\n")
}

func inputView(view string, focused bool) string {
	if focused {
		return inputFocusedStyle.Render(view)
	}

	return inputStyle.Render(view)
}
