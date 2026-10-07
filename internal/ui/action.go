package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// ActionMsg is a key press that navigation didn't handle, meant for screen actions. While the screen is typing, only
// keys that can't be text and match its actions come as ActionMsg. Text inputs never see it. They receive raw
// tea.KeyPressMsg.
type ActionMsg struct {
	Key tea.KeyPressMsg
}

func (m ActionMsg) String() string {
	return m.Key.String()
}

// Action binds a key to a message for the screen. Actions without message only describe keys handled
// by navigation, like enter or l, so that they show up in the footer.
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
