package ui

import tea "github.com/charmbracelet/bubbletea"

// Label is a plain string shown in the UI. It can be a non-interactive List item.
type Label struct {
	title string
}

// NewLabel returns new Label from string.
func NewLabel(title string) *Label {
	return &Label{
		title: title,
	}
}

func (l *Label) Init() tea.Cmd {
	return nil
}

func (l *Label) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return l, nil
}

func (l *Label) View() string {
	return l.title
}

// Set sets label's value.
func (l *Label) Set(value string) {
	l.title = value
}

// Value returns the content of Label.
func (l *Label) Value() string {
	return l.title
}
