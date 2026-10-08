package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// noticeTime is how long a notice stays in the status.
const noticeTime = 4 * time.Second

// Status is a short line about the state of a screen, shown in the header. A busy status is work in progress, like
// loading, and shows a spinner.
type Status struct {
	text string
	busy bool

	// gen counts changes of the status, so that an expiring notice clears only itself
	gen int
}

// NewStatus returns empty Status.
func NewStatus() *Status {
	return &Status{}
}

// Set shows text, like a failure or a notice.
func (s *Status) Set(text string) {
	s.text, s.busy = text, false
	s.gen++
}

// Busy shows text as work in progress until the next Set or Busy.
func (s *Status) Busy(text string) {
	s.text, s.busy = text, true
	s.gen++
}

// Notice shows text muted for a few seconds. The command it returns clears the text then, unless something else
// replaced it.
func (s *Status) Notice(text string) tea.Cmd {
	s.Set(MutedStyle.Render(text))
	gen := s.gen
	return tea.Tick(noticeTime, func(time.Time) tea.Msg {
		return ExpireMsg{status: s, gen: gen}
	})
}

// ExpireMsg ends a notice. The router applies it, so it reaches the status of any screen.
type ExpireMsg struct {
	status *Status
	gen    int
}

// Apply clears the notice when the status still shows it.
func (m ExpireMsg) Apply() {
	if m.status.gen == m.gen {
		m.status.Set("")
	}
}

// Value returns the text, empty when there is nothing to report.
func (s *Status) Value() string {
	return s.text
}

// Loading reports whether the status is work in progress.
func (s *Status) Loading() bool {
	return s.busy
}
