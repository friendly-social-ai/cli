package ui

import (
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
)

// Markdown renders markdown with glamour in the theme colors. It keeps one renderer for the last wrap width. Like
// the web, it keeps single line breaks, turns emoji shortcodes into emoji and shows LaTeX math, here as Unicode.
type Markdown struct {
	style    ansi.StyleConfig
	width    int
	renderer *glamour.TermRenderer
}

// NewMarkdown creates Markdown in the colors that SetTheme picked.
func NewMarkdown() *Markdown {
	style := styles.LightStyleConfig
	if palette.dark {
		style = styles.DarkStyleConfig
	}

	// text sits inside the program layout, so it gets no margins, no blank lines around it and no color of its own
	style.Document.BlockPrefix, style.Document.BlockSuffix = "", ""
	style.Document.Color = nil
	style.Document.Margin = new(uint)

	primary, muted := palette.primary, palette.muted
	style.Heading.Color = &primary
	style.H1.Color, style.H1.BackgroundColor = &primary, nil
	style.H1.Prefix, style.H1.Suffix = "# ", ""
	style.LinkText.Color = &primary
	style.Link.Color = &muted

	return &Markdown{style: style}
}

// Render returns text rendered as markdown and wrapped to width, or text itself when it fails to render.
func (m *Markdown) Render(text string, width int) string {
	if m.renderer == nil || m.width != width {
		renderer, err := glamour.NewTermRenderer(
			glamour.WithStyles(m.style),
			glamour.WithWordWrap(width),
			glamour.WithPreservedNewLines(),
			glamour.WithEmoji())
		if err != nil {
			return text
		}

		m.renderer, m.width = renderer, width
	}

	out, err := m.renderer.Render(mathText(text))
	if err != nil {
		return text
	}

	return strings.Trim(out, "\n")
}
