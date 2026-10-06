package screen

import tea "github.com/charmbracelet/bubbletea"

// Send returns command delivering msg.
func Send(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

// ChangeMsg singals that router must change the current screen.
type ChangeMsg struct {
	NewType Type
}

// ErrorMsg is a message that represents an error occured in program.
type ErrorMsg struct {
	Value error
}

// TickMsg signalizes that screen must be updated.
type TickMsg struct{}
