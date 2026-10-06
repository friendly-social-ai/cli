package screen

import tea "charm.land/bubbletea/v2"

// Send returns command delivering msg.
func Send(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

// ChangeMsg signals that router must change the current screen.
type ChangeMsg struct {
	NewType Type
}

// ErrorMsg carries an error that occurred in the program.
type ErrorMsg struct {
	Value error
}

// TickMsg signals that screen must be redrawn.
type TickMsg struct{}

// MinuteMsg reaches every screen once a minute, to redraw relative times and check for new activity.
type MinuteMsg struct{}
