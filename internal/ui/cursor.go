package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// markStart and markEnd wrap the shape in the cursor mark. The mark is an APC sequence, which takes no cells, so width
// counts and truncation ignore it.
const (
	markStart = "\x1b_cursor;"
	markEnd   = "\x1b\\"
)

// cursorMark returns the mark an Editor leaves where the terminal cursor goes.
func cursorMark(shape tea.CursorShape) string {
	return markStart + strconv.Itoa(int(shape)) + markEnd
}

// TakeCursor removes the cursor mark from frame and returns a terminal cursor at the mark's cell, or nil when frame has
// no mark. The cursor does not blink, as in Neovim.
func TakeCursor(frame string) (string, *tea.Cursor) {
	at := strings.Index(frame, markStart)
	if at < 0 {
		return frame, nil
	}

	before, rest := frame[:at], frame[at+len(markStart):]
	shape, rest, _ := strings.Cut(rest, markEnd)
	n, _ := strconv.Atoi(shape)
	line := before[strings.LastIndexByte(before, '\n')+1:]
	cursor := tea.NewCursor(ansi.StringWidth(line), strings.Count(before, "\n"))
	cursor.Shape, cursor.Blink = tea.CursorShape(n), false
	return before + rest, cursor
}
