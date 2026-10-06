package ui

import (
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Field is an abstraction over textinput.Model for embedding it into ui package contract.
type Field struct {
	input *textinput.Model
}

// NewField creates new Field based on provided textinput.Model.
func NewField(input textinput.Model) *Field {
	input.Blur()
	input.PlaceholderStyle = MutedStyle
	return &Field{
		input: &input,
	}
}

func (f *Field) Init() tea.Cmd {
	return nil
}

func (f *Field) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case FocusMsg:
		return f, tea.Batch(
			f.input.Focus(),
			f.input.Cursor.SetMode(cursor.CursorBlink),
		)
	case UnfocusMsg:
		f.input.Blur()
		return f, f.input.Cursor.SetMode(cursor.CursorStatic)
	}

	model, cmd := f.input.Update(msg)
	*f.input = model
	return f, cmd
}

func (f *Field) View() string {
	return inputView(f.input.View(), f.input.Focused())
}

// Value returns current filled string.
func (f *Field) Value() string {
	return f.input.Value()
}

// Raw returns underlying textinput.Model.
func (f *Field) Raw() *textinput.Model {
	return f.input
}
