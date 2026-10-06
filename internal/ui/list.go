package ui

import (
	"fmt"
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

	// inner is the first visible line of the selected item when it is clipped.
	inner int

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
	l.inner = 0
	l.Set(items...)
}

// Cursor returns index of the selected item.
func (l *List) Cursor() int {
	return l.cursor
}

// Select moves cursor to item i.
func (l *List) Select(i int) {
	l.move(i)
}

func (l *List) move(i int) tea.Cmd {
	if i < 0 || i >= len(l.items) || i == l.cursor {
		return nil
	}

	cmds := make([]tea.Cmd, 2)
	l.items[l.cursor], cmds[0] = l.items[l.cursor].Update(UnselectMsg{})
	l.cursor = i
	l.items[l.cursor], cmds[1] = l.items[l.cursor].Update(SelectMsg{})
	l.inner = 0

	return tea.Batch(cmds...)
}

// clip returns height at which items get clipped, half of the visible list. Zero means no clipping.
func (l *List) clip() int {
	if l.height <= 0 {
		return 0
	}

	return max(l.height/2, 5)
}

// Scrollable reports whether the selected item is clipped and can be scrolled with ScrollMsg.
func (l *List) Scrollable() bool {
	return len(l.items) > 0 && l.clip() > 0 && lipgloss.Height(l.items[l.cursor].View()) > l.clip()
}

// itemView renders item i, clipping it to clip lines with a position indicator when it is taller.
func (l *List) itemView(i int) string {
	view := l.items[i].View()
	c := l.clip()
	all := strings.Split(view, "\n")
	if c == 0 || len(all) <= c {
		return view
	}

	start := 0
	if i == l.cursor {
		start = min(l.inner, len(all)-c)
	}

	indicator := MutedStyle.Render(fmt.Sprintf("lines %d-%d of %d", start+1, start+c, len(all)))
	return strings.Join(all[start:start+c], "\n") + "\n" + indicator
}

// Len returns number of items.
func (l *List) Len() int {
	return len(l.items)
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
		switch msg.Direction {
		case DirectionDown:
			return l, l.move(l.cursor + 1)
		case DirectionUp:
			return l, l.move(l.cursor - 1)
		}

		return l, nil
	case ScrollMsg:
		// half of the clipped window per step, like ctrl+d and ctrl+u in vim
		if l.Scrollable() {
			step := max(l.clip()/2, 1)
			if msg.Direction == DirectionUp {
				step = -step
			}

			l.inner = max(min(l.inner+step, lipgloss.Height(l.items[l.cursor].View())-l.clip()), 0)
		}

		return l, nil
	}

	var cmd tea.Cmd
	l.items[l.cursor], cmd = l.items[l.cursor].Update(msg)
	return l, cmd
}

func (l *List) View() string {
	if len(l.items) == 0 {
		return ""
	}

	views := make([]string, len(l.items))
	for i := range l.items {
		prefix := "  "
		if l.cursor == i {
			prefix = listMarker
		}

		lines := strings.Split(l.itemView(i), "\n")
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
