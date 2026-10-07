package ui

// Status is a short line about the state of a screen, shown in the header. A busy status is work in progress, like
// loading, and shows a spinner.
type Status struct {
	text string
	busy bool
}

// NewStatus returns empty Status.
func NewStatus() *Status {
	return &Status{}
}

// Set shows text, like a failure or a notice.
func (s *Status) Set(text string) {
	s.text, s.busy = text, false
}

// Busy shows text as work in progress until the next Set or Busy.
func (s *Status) Busy(text string) {
	s.text, s.busy = text, true
}

// Value returns the text, empty when there is nothing to report.
func (s *Status) Value() string {
	return s.text
}

// Loading reports whether the status is work in progress.
func (s *Status) Loading() bool {
	return s.busy
}
