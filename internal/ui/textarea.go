package ui

import (
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

	model, cmd := a.input.Update(msg)
	*a.input = model
	return a, cmd
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
