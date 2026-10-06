package screen

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Type represents type of the current screen and serves as an identificator.
type Type string

const (
	TypeRegister  Type = "register"
	TypeHome      Type = "home"
	TypeProfile   Type = "profile"
	TypeAuth      Type = "auth"
	TypePeople    Type = "people"
	TypeCommunity Type = "community"
)

// Model represents Screen which is basically an extended tea.Model.
type Model interface {
	ID() Type

	// Keys returns key bindings currently available on the screen, shown in the footer.
	Keys() []key.Binding

	Init() tea.Cmd
	Update(tea.Msg) (Model, tea.Cmd)
	View() string
}
