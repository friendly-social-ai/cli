package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Colors follow the Friendly web theme, adjusted for text contrast on light and dark terminals. SetTheme and LoadTheme
// pick them.
var (
	ColorPrimary color.Color
	ColorMuted   color.Color
	ColorDanger  color.Color
	ColorBorder  color.Color
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
	// selection is the escape sequence of the background behind the selected item
	selection string
)

func init() {
	SetTheme(true)
}

// colors are the theme colors for one terminal background, as hex strings or ANSI numbers. Selection is the background
// of the selected item.
type colors struct {
	primary, muted, danger, border, selection string
}

// lightColors and darkColors are the default themes for light and dark terminal backgrounds.
var (
	lightColors = colors{
		primary: "#0060D0", muted: "#646464", danger: "#C4391D", border: "#C8C8C8", selection: "#E8EDF9",
	}
	darkColors = colors{
		primary: "#6E8BFF", muted: "#B4B4B4", danger: "#F0715A", border: "#4E4E4E", selection: "#232A3B",
	}
)

// palette holds the theme as hex strings for libraries that take colors as strings, like glamour.
var palette struct {
	dark    bool
	primary string
	muted   string
}

// SetTheme picks the default colors for dark or light terminal background. Call it before building the UI, since
// rendered text keeps the colors it was rendered with.
func SetTheme(dark bool) {
	c := lightColors
	if dark {
		c = darkColors
	}

	apply(dark, c)
}

func apply(dark bool, c colors) {
	palette.dark = dark
	palette.primary = c.primary
	palette.muted = c.muted

	ColorPrimary = lipgloss.Color(c.primary)
	ColorMuted = lipgloss.Color(c.muted)
	ColorDanger = lipgloss.Color(c.danger)
	ColorBorder = lipgloss.Color(c.border)

	MutedStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	AccentStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	DangerStyle = lipgloss.NewStyle().Foreground(ColorDanger)

	inputStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	inputFocusedStyle = inputStyle.BorderForeground(ColorPrimary)
	listMarker = lipgloss.NewStyle().Foreground(ColorPrimary).Render("▎ ")
	selection = ansi.Style{}.BackgroundColor(lipgloss.Color(c.selection)).String()
}

// highlight puts the selection background behind line, padded to width. A reset inside the line would end the
// background early, so the background starts again after each one.
func highlight(line string, width int) string {
	line += strings.Repeat(" ", max(width-lipgloss.Width(line), 0))
	return selection + strings.ReplaceAll(line, ansi.ResetStyle, ansi.ResetStyle+selection) + ansi.ResetStyle
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

// Empty centers title in a width by height area, with key k and desc below it as a hint. Screens show it when they
// have nothing to list.
func Empty(width, height int, title, k, desc string) string {
	// an unbound key has no hint
	if k == "" {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, BoldStyle.Render(title))
	}

	hint := AccentStyle.Render(k) + MutedStyle.Render(" "+desc)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, BoldStyle.Render(title), "", hint))
}

func inputView(view string, focused bool) string {
	if focused {
		return inputFocusedStyle.Render(view)
	}

	return inputStyle.Render(view)
}
