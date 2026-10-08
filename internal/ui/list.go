package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// List represents collection of elements that you can select and interact with.
type List struct {
	cursor int
	items  []Component

	// height limits rendered lines, and the list scrolls to keep the cursor visible. Zero means unlimited.
	height int
	offset int

	// inner is the first visible line of the selected item when it is clipped.
	inner int

	// gap is the number of blank lines between items.
	gap int

	// attachment is a view that View draws under the selected item without its highlight, like a reply box.
	attachment string

	// width is the width of the list. The background of the selected item spans it. Zero means no background, for lists
	// like forms that mark the selection another way.
	width int

	// top is the line of the screen where the list starts. rows holds the index of the item drawn on each line of the
	// last View, or -1 for a gap line. ClickMsg uses both to find the clicked item.
	top  int
	rows []int
}

// NewList creates new List based on the list of items.
func NewList(items ...Component) *List {
	return &List{
		items: items,
	}
}

// Set replaces items keeping cursor position where possible and selects the item under cursor.
func (l *List) Set(items ...Component) {
	l.items = items
	l.cursor = max(min(l.cursor, len(items)-1), 0)
	l.offset = min(l.offset, l.cursor)
	if len(items) > 0 {
		l.items[l.cursor], _ = l.items[l.cursor].Update(SelectMsg{})
	}
}

// Reset replaces items moving cursor to the first one.
func (l *List) Reset(items ...Component) {
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

// SelectFocused moves cursor to item i and focus with it, so typing goes on in item i.
func (l *List) SelectFocused(i int) tea.Cmd {
	l.items[l.cursor], _ = l.items[l.cursor].Update(UnfocusMsg{})
	l.move(i)

	var cmd tea.Cmd
	l.items[l.cursor], cmd = l.items[l.cursor].Update(FocusMsg{})
	return cmd
}

// Position returns the selected item and the first visible item, to restore them later with SetPosition.
func (l *List) Position() (cursor, offset int) {
	return l.cursor, l.offset
}

// SetPosition selects item cursor and scrolls to show item offset first, both clamped to the items.
func (l *List) SetPosition(cursor, offset int) {
	l.move(max(min(cursor, len(l.items)-1), 0))
	l.offset = max(min(offset, l.cursor), 0)
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

// clip returns the clipping height for items, half of the visible list. Zero means no clipping. Items clip shorter
// when the attachment would otherwise run off the bottom of the list.
func (l *List) clip() int {
	if l.height <= 0 {
		return 0
	}

	c := max(l.height/2, 5)
	if l.attachment != "" {
		// leave one line for the position indicator
		c = max(min(c, l.height-lipgloss.Height(l.attachment)-1), 1)
	}

	return c
}

// attached returns the number of lines the attachment adds under item i.
func (l *List) attached(i int) int {
	if i != l.cursor || l.attachment == "" {
		return 0
	}

	return lipgloss.Height(l.attachment)
}

// Scrollable reports whether the selected item is clipped, so ScrollMsg can scroll it.
func (l *List) Scrollable() bool {
	return len(l.items) > 0 && l.clip() > 0 && lipgloss.Height(l.items[l.cursor].View()) > l.clip()
}

// itemView renders item i. A taller item is cut to clip lines and gets a position indicator.
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

// wheel scrolls lines of the selected item while it is clipped, then the list by one item. The cursor follows only
// to stay on screen.
func (l *List) wheel(direction Direction) tea.Cmd {
	if l.height <= 0 {
		return nil
	}

	if l.scrollItem(direction, 1) {
		return nil
	}

	if direction == DirectionDown {
		if l.AtEnd() {
			return nil
		}

		l.offset++
		return l.move(max(l.cursor, l.offset))
	}

	if l.offset == 0 {
		return nil
	}

	// the cursor item has to fit below the new offset, as View requires
	l.offset--
	cursor := l.cursor
	for cursor > l.offset && l.span(l.offset, cursor+1) > l.height {
		cursor--
	}

	return l.move(cursor)
}

// page scrolls the selected item by half of the clipping height while it is clipped. Once the item shows its end, page
// moves the list and the cursor together by the items that fill half of the list height, like CTRL-D in vim.
func (l *List) page(direction Direction) tea.Cmd {
	if l.height <= 0 {
		return nil
	}

	if l.scrollItem(direction, max(l.clip()/2, 1)) {
		return nil
	}

	// count items from the cursor until they fill half of the height
	d := 1
	if direction == DirectionUp {
		d = -1
	}

	n, lines := 0, 0
	for i := l.cursor + d; i >= 0 && i < len(l.items) && lines < l.height/2; i += d {
		lines += l.span(i, i+1)
		n++
	}

	if direction == DirectionUp {
		l.offset = max(l.offset-n, 0)
		return l.move(l.cursor - n)
	}

	for range n {
		if l.AtEnd() {
			break
		}

		l.offset++
	}

	return l.move(l.cursor + n)
}

// scrollItem scrolls the selected item by step lines while it is clipped and has lines left in direction. It reports
// whether it scrolled.
func (l *List) scrollItem(direction Direction, step int) bool {
	if !l.Scrollable() {
		return false
	}

	last := lipgloss.Height(l.items[l.cursor].View()) - l.clip()
	switch {
	case direction == DirectionDown && l.inner < last:
		l.inner = min(l.inner+step, last)
	case direction == DirectionUp && l.inner > 0:
		l.inner = max(l.inner-step, 0)
	default:
		return false
	}

	return true
}

// span returns the number of lines that items from first up to end take in View, with the gaps after them.
func (l *List) span(first, end int) int {
	total := 0
	for i := first; i < end; i++ {
		total += lipgloss.Height(l.itemView(i)) + l.attached(i)
		if i < len(l.items)-1 {
			total += l.gap
		}
	}

	return total
}

// AtEnd reports whether the last item is on screen, so the list can't scroll further down.
func (l *List) AtEnd() bool {
	return l.height <= 0 || l.span(l.offset, len(l.items)) <= l.height
}

// Len returns number of items.
func (l *List) Len() int {
	return len(l.items)
}

// SetGap sets number of blank lines between items.
func (l *List) SetGap(gap int) {
	l.gap = gap
}

// SetAttachment sets the view drawn under the selected item, "" for none.
func (l *List) SetAttachment(view string) {
	l.attachment = view
}

// SetWidth sets the width that the background of the selected item spans.
func (l *List) SetWidth(width int) {
	l.width = width
}

// SetHeight limits List to provided number of lines.
func (l *List) SetHeight(height int) {
	l.height = height
}

// SetTop sets the line of the screen where List starts, so that ClickMsg finds the item under the click.
func (l *List) SetTop(top int) {
	l.top = top
}

func (l *List) Update(msg tea.Msg) (Component, tea.Cmd) {
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
	case JumpMsg:
		if msg.Direction == DirectionUp {
			return l, l.move(0)
		}

		return l, l.move(len(l.items) - 1)
	case ScrollMsg:
		return l, l.page(msg.Direction)
	case ClickMsg:
		y := msg.Y - l.top
		if y < 0 || y >= len(l.rows) || l.rows[y] < 0 {
			return l, nil
		}

		i := l.rows[y]
		again := i == l.cursor
		return l, tea.Batch(l.move(i), func() tea.Msg {
			return ClickedMsg{Again: again}
		})
	case WheelMsg:
		return l, l.wheel(msg.Direction)
	}

	var cmd tea.Cmd
	l.items[l.cursor], cmd = l.items[l.cursor].Update(msg)
	return l, cmd
}

func (l *List) View() string {
	l.rows = l.rows[:0]
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
			if l.cursor == i && l.width > 0 {
				lines[j] = highlight(lines[j], l.width-lipgloss.Width(listMarker))
			}

			lines[j] = prefix + lines[j]
		}

		if l.attached(i) > 0 {
			for line := range strings.SplitSeq(l.attachment, "\n") {
				lines = append(lines, "  "+line)
			}
		}

		views[i] = strings.Join(lines, "\n")
		if i < len(l.items)-1 {
			views[i] += strings.Repeat("\n", l.gap)
		}
	}

	if l.height <= 0 {
		l.record(views, 0)
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

	l.record(views, l.offset)
	l.rows = l.rows[:min(len(l.rows), l.height)]
	return strings.Join(result[:min(len(result), l.height)], "\n")
}

// record appends the item index of each line of views to rows, starting at item first. Lines of the attachment and
// gaps get -1.
func (l *List) record(views []string, first int) {
	for i := first; i < len(views); i++ {
		n := lipgloss.Height(views[i])
		gap := 0
		if i < len(views)-1 {
			gap = l.gap
		}

		for range n - gap - l.attached(i) {
			l.rows = append(l.rows, i)
		}

		for range l.attached(i) + gap {
			l.rows = append(l.rows, -1)
		}
	}
}

func lines(views []string) int {
	total := 0
	for _, view := range views {
		total += lipgloss.Height(view)
	}

	return total
}
