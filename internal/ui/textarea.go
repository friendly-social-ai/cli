package ui

import (
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TextArea is an abstraction over textarea.Model for embedding multi-line input into ui package contract.
type TextArea struct {
	input *textarea.Model
}

// NewTextArea creates new TextArea based on provided textarea.Model.
func NewTextArea(input textarea.Model) *TextArea {
	input.Blur()
	for _, style := range []*textarea.Style{&input.FocusedStyle, &input.BlurredStyle} {
		style.CursorLine = lipgloss.NewStyle()
		style.Placeholder = MutedStyle
		style.EndOfBuffer = lipgloss.NewStyle()
	}

	return &TextArea{
		input: &input,
	}
}

func (a *TextArea) Init() tea.Cmd {
	return nil
}

func (a *TextArea) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case FocusMsg:
		return a, tea.Batch(
			a.input.Focus(),
			a.input.Cursor.SetMode(cursor.CursorBlink),
		)
	case UnfocusMsg:
		a.input.Blur()
		return a, a.input.Cursor.SetMode(cursor.CursorStatic)
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
