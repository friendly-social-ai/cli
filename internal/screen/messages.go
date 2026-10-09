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

// MinuteMsg reaches every screen once a minute, to redraw relative times.
type MinuteMsg struct{}

// PollMsg reaches every screen at the refresh the user sets, to check for new posts and activity.
type PollMsg struct{}

// ReselectMsg reaches a tab screen when the user picks its tab while already on it, to go back to its first view.
type ReselectMsg struct{}

// ShownMsg reaches a screen when it becomes the current one.
type ShownMsg struct{}
