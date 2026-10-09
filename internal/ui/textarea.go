package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// TextArea is an abstraction over textarea.Model for embedding multi-line input into ui package contract.
type TextArea struct {
	input *textarea.Model
}

// NewTextArea creates new TextArea based on provided textarea.Model.
func NewTextArea(input textarea.Model) *TextArea {
	input.Blur()
	styles := input.Styles()
	for _, state := range []*textarea.StyleState{&styles.Focused, &styles.Blurred} {
		state.CursorLine = lipgloss.NewStyle()
		state.Placeholder = MutedStyle
		state.EndOfBuffer = lipgloss.NewStyle()
	}
	input.SetStyles(styles)

	return &TextArea{
		input: &input,
	}
}

func (a *TextArea) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg.(type) {
	case FocusMsg:
		return a, a.input.Focus()
	case UnfocusMsg:
		a.input.Blur()
		return a, nil
	}

	k, newline := msg.(tea.KeyPressMsg)
	newline = newline && key.Matches(k, a.input.KeyMap.InsertNewline)
	// enter deletes the selection before it adds a line, so count the lines without the selected ones
	lines := a.input.LineCount()
	if start, end, ok := a.input.Selection(); ok {
		lines -= end.Row - start.Row
	}

	model, cmd := a.input.Update(msg)
	*a.input = model

	// a new line starts with the indentation of the line above it
	if newline && a.input.LineCount() > lines {
		above := strings.Split(a.input.Value(), "\n")[a.input.Line()-1]
		a.input.InsertString(above[:len(above)-len(strings.TrimLeft(above, " "))])
	}

	return a, cmd
}

// indent is the width of one level of indentation. textarea also turns a pasted tab into this many spaces.
const indent = 4

// Indent adds a level of indentation to the lines the selection covers. Without a selection it adds spaces at the
// cursor up to the next level.
func (a *TextArea) Indent() {
	if _, _, ok := a.input.Selection(); !ok {
		a.input.InsertString(strings.Repeat(" ", indent-a.input.Column()%indent))
		return
	}

	a.shiftLines(func(string) int { return indent })
}

// Outdent removes up to a level of indentation from the lines the selection covers, or from the line of the cursor.
func (a *TextArea) Outdent() {
	a.shiftLines(func(line string) int {
		return -min(indent, len(line)-len(strings.TrimLeft(line, " ")))
	})
}

// shiftLines adds delta(line) spaces to the start of each line the selection covers, or of the cursor's line. A
// negative delta removes spaces. A selection ending at the start of a line leaves that line alone. The cursor and the
// selection stay on the same text. Nothing changes when the result would pass CharLimit.
func (a *TextArea) shiftLines(delta func(string) int) {
	cursor := textarea.Position{Row: a.input.Line(), Col: a.input.Column()}
	start, end, selected := a.input.Selection()
	if !selected {
		start, end = cursor, cursor
	}

	// the cursor is at one end of the selection and the anchor at the other
	anchor := start
	if anchor == cursor {
		anchor = end
	}

	last := end.Row
	if end.Row > start.Row && end.Col == 0 {
		last--
	}

	lines := strings.Split(a.input.Value(), "\n")
	shifts := make(map[int]int)
	length := a.input.Length()
	for row := start.Row; row <= last; row++ {
		n := delta(lines[row])
		shifts[row] = n
		length += n
		if n > 0 {
			lines[row] = strings.Repeat(" ", n) + lines[row]
		} else {
			lines[row] = lines[row][-n:]
		}
	}

	if a.input.CharLimit > 0 && length > a.input.CharLimit {
		return
	}

	// a position at the start of a line stays there, so a selection of whole lines keeps them whole
	shifted := func(p textarea.Position) textarea.Position {
		if p.Col > 0 {
			p.Col = max(p.Col+shifts[p.Row], 0)
		}

		return p
	}

	a.input.SetValue(strings.Join(lines, "\n"))
	a.moveTo(shifted(anchor))
	if selected {
		a.selectTo(shifted(cursor))
	}
}

// moveTo puts the cursor at p.
func (a *TextArea) moveTo(p textarea.Position) {
	for a.input.Line() > p.Row {
		a.input.CursorUp()
	}

	for a.input.Line() < p.Row {
		a.input.CursorDown()
	}

	a.input.SetCursorColumn(p.Col)
}

// selectTo selects from the cursor to p. textarea has no method to set a selection, so selectTo sends it shift+arrow
// key presses. textarea ignores keys while unfocused, so selectTo does nothing then.
func (a *TextArea) selectTo(p textarea.Position) {
	if !a.input.Focused() {
		return
	}

	press := func(code rune) {
		model, _ := a.input.Update(tea.KeyPressMsg{Code: code, Mod: tea.ModShift})
		*a.input = model
	}

	for a.input.Line() < p.Row {
		press(tea.KeyDown)
	}

	for a.input.Line() > p.Row {
		press(tea.KeyUp)
	}

	for a.input.Column() < p.Col {
		press(tea.KeyRight)
	}

	for a.input.Column() > p.Col {
		press(tea.KeyLeft)
	}
}

func (a *TextArea) View() string {
	return inputView(a.input.View(), a.input.Focused())
}

// Value returns current filled string.
func (a *TextArea) Value() string {
	return a.input.Value()
}

// Raw returns underlying textarea.Model.
func (a *TextArea) Raw() *textarea.Model {
	return a.input
}
