package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// listMarker is drawn beside every line of the selected item, unselected items are indented by its width.
var listMarker = lipgloss.NewStyle().Foreground(ColorPrimary).Render("▎ ")

// List represents collection of elements that you can select and interact with.
type List struct {
	cursor int
	items  []tea.Model

	// height limits rendered lines, scrolling to keep cursor visible. Zero means unlimited.
	height int
	offset int

	// gap is the number of blank lines between items.
	gap int
}

// NewList creates new List based on the list of items.
func NewList(items ...tea.Model) *List {
	return &List{
		items: items,
	}
}

// Set replaces items keeping cursor position where possible and selects the item under cursor.
func (l *List) Set(items ...tea.Model) {
	l.items = items
	l.cursor = max(min(l.cursor, len(items)-1), 0)
	l.offset = min(l.offset, l.cursor)
	if len(items) > 0 {
		l.items[l.cursor], _ = l.items[l.cursor].Update(SelectMsg{})
	}
}

// Reset replaces items moving cursor to the first one.
func (l *List) Reset(items ...tea.Model) {
	l.cursor = 0
	l.offset = 0
	l.Set(items...)
}

// Cursor returns index of the selected item.
func (l *List) Cursor() int {
	return l.cursor
}

// SetGap sets number of blank lines between items.
func (l *List) SetGap(gap int) {
	l.gap = gap
}

// SetHeight limits List to provided number of lines.
func (l *List) SetHeight(height int) {
	l.height = height
}

func (l *List) Init() tea.Cmd {
	return nil
}

func (l *List) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(l.items) == 0 {
		return l, nil
	}

	switch msg := msg.(type) {
	case MoveMsg:
		cmds := make([]tea.Cmd, 2)
		l.items[l.cursor], cmds[0] = l.items[l.cursor].Update(UnselectMsg{})

		if msg.Direction == DirectionDown {
			l.cursor = min(l.cursor+1, len(l.items)-1)
		}

		if msg.Direction == DirectionUp {
			l.cursor = max(l.cursor-1, 0)
		}

		l.items[l.cursor], cmds[1] = l.items[l.cursor].Update(SelectMsg{})
		return l, tea.Batch(cmds...)
	}

	var cmd tea.Cmd
	l.items[l.cursor], cmd = l.items[l.cursor].Update(msg)
	return l, cmd
}

func (l *List) View() string {
	views := make([]string, len(l.items))
	for i, input := range l.items {
		prefix := "  "
		if l.cursor == i {
			prefix = listMarker
		}

		lines := strings.Split(input.View(), "\n")
		for j := range lines {
			lines[j] = prefix + lines[j]
		}

		views[i] = strings.Join(lines, "\n")
		if i < len(l.items)-1 {
			views[i] += strings.Repeat("\n", l.gap)
		}
	}

	if l.height <= 0 {
		return lipgloss.JoinVertical(lipgloss.Left, views...)
	}

	// scroll so that the whole cursor item fits, then fill the rest of height below it
	l.offset = min(l.offset, l.cursor)
	for l.offset < l.cursor && lines(views[l.offset:l.cursor+1]) > l.height {
		l.offset++
	}

	var result []string
	for _, view := range views[l.offset:] {
		result = append(result, strings.Split(view, "\n")...)
		if len(result) >= l.height {
			break
		}
	}

	return strings.Join(result[:min(len(result), l.height)], "\n")
}

func lines(views []string) int {
	total := 0
	for _, view := range views {
		total += lipgloss.Height(view)
	}

	return total
}
