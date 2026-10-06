package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Button is an implementation of button, with which you can interact, and which can be either selected or not.
type Button struct {
	selected bool

	title  string
	action tea.Cmd
}

// NewButton creates new Button instance with provided title and action that will be returned on interaction.
func NewButton(title string, action tea.Cmd) *Button {
	return &Button{
		title:  title,
		action: action,
	}
}

func (b *Button) Init() tea.Cmd {
	return nil
}

func (b *Button) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case SelectMsg:
		b.selected = true
	case UnselectMsg:
		b.selected = false
	case InteractMsg:
		return b, b.action
	}

	return b, nil
}

func (b *Button) View() string {
	if b.selected {
		return AccentStyle.Render(b.title)
	}

	return b.title
}
