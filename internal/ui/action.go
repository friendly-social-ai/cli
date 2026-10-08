package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// ActionMsg is a key press that navigation didn't handle, meant for screen actions. Keys is the key as bubbletea
// writes it, or the keys of a sequence joined by spaces, like "g t". While the screen is typing, only keys that can't
// be text and match its actions come as ActionMsg. Text inputs never see it. They receive raw tea.KeyPressMsg.
type ActionMsg struct {
	Keys string
}

func (m ActionMsg) String() string {
	return m.Keys
}

// Action binds a key to a message for the screen. Actions without message only describe keys handled
// by navigation, like enter or l, so that they show up in the footer.
type Action struct {
	Key key.Binding
	Msg tea.Msg
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
