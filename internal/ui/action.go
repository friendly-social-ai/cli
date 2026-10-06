package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ActionMsg is a key pressed in normal mode that navigation didn't handle, meant for screen actions.
// Text inputs never see it. They receive raw tea.KeyMsg only in insert mode.
type ActionMsg struct {
	Key tea.KeyMsg
}

func (m ActionMsg) String() string {
	return m.Key.String()
}

// Action binds a key to a message for the screen. Actions without message only describe keys handled
// by navigation, like enter or i, so that they show up in the footer.
type Action struct {
	Key key.Binding
	Msg tea.Msg
}

// Key creates binding of key k with help description. Alternative keys trigger it too without being shown.
func Key(k, desc string, alts ...string) key.Binding {
	return key.NewBinding(key.WithKeys(append([]string{k}, alts...)...), key.WithHelp(k, desc))
}

// Keys returns bindings of actions.
func Keys(actions []Action) []key.Binding {
	bindings := make([]key.Binding, len(actions))
	for i, action := range actions {
		bindings[i] = action.Key
	}

	return bindings
}

// Dispatch returns message of the action bound to pressed key, or nil when there is none.
func Dispatch(actions []Action, msg ActionMsg) tea.Msg {
	for _, action := range actions {
		if action.Msg != nil && key.Matches(msg, action.Key) {
			return action.Msg
		}
	}

	return nil
}
