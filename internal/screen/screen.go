package screen

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Type identifies a screen.
type Type string

const (
	TypeRegister  Type = "register"
	TypeHome      Type = "home"
	TypeProfile   Type = "profile"
	TypeAuth      Type = "auth"
	TypePeople    Type = "people"
	TypeCommunity Type = "community"
	TypeActivity  Type = "activity"
)

// Model is a screen, an extended tea.Model with an ID and key bindings.
type Model interface {
	ID() Type

	// Keys returns key bindings currently available on the screen, shown in the footer.
	Keys() []key.Binding

	Init() tea.Cmd
	Update(tea.Msg) (Model, tea.Cmd)
	View() string
}
