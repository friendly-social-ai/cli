package ui

import (
	tea "charm.land/bubbletea/v2"
)

// Button is an interactive button that can be selected.
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

// SetTitle replaces title of the button.
func (b *Button) SetTitle(title string) {
	b.title = title
}

func (b *Button) Update(msg tea.Msg) (Component, tea.Cmd) {
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
