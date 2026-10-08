package ui

import (
	tea "charm.land/bubbletea/v2"
)

// Button is an interactive button. It renders the same whether selected or not, since the list marks the selection.
type Button struct {
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

// SetTitle replaces title of the button.
func (b *Button) SetTitle(title string) {
	b.title = title
}

func (b *Button) Update(msg tea.Msg) (Component, tea.Cmd) {
	if _, ok := msg.(InteractMsg); ok {
		return b, b.action
	}

	return b, nil
}

func (b *Button) View() string {
	return b.title
}
