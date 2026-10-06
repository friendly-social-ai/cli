package ui

import tea "charm.land/bubbletea/v2"

// Component is a part of UI that List can hold.
type Component interface {
	Update(tea.Msg) (Component, tea.Cmd)
	View() string
}
