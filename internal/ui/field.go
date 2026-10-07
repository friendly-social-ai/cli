package ui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// Field is an abstraction over textinput.Model for embedding it into ui package contract.
type Field struct {
	input *textinput.Model
}

// NewField creates new Field based on provided textinput.Model.
func NewField(input textinput.Model) *Field {
	input.Blur()
	styles := input.Styles()
	styles.Focused.Placeholder = MutedStyle
	styles.Blurred.Placeholder = MutedStyle
	input.SetStyles(styles)

	return &Field{
		input: &input,
	}
}

func (f *Field) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg.(type) {
	case FocusMsg:
		return f, f.input.Focus()
	case UnfocusMsg:
		f.input.Blur()
		return f, nil
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

// Filled reports whether any of fields has text.
func Filled(fields []*Field) bool {
	for _, field := range fields {
		if field.Value() != "" {
			return true
		}
	}

	return false
}

// Raw returns underlying textinput.Model.
func (f *Field) Raw() *textinput.Model {
	return f.input
}
