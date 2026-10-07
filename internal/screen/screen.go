package screen

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/friendly-social/cli/internal/ui"
)

// Type identifies a screen.
type Type string

const (
	TypeRegister  Type = "register"
	TypeHome      Type = "home"
	TypeProfile   Type = "profile"
	TypeAuth      Type = "login"
	TypePeople    Type = "people"
	TypeCommunity Type = "community"
	TypeActivity  Type = "activity"
	TypeUser      Type = "user"
)

// Model is a screen, an extended tea.Model with an ID and key bindings.
type Model interface {
	ID() Type

	// Keys returns key bindings currently available on the screen, shown in the footer.
	Keys() []key.Binding

	// Status returns a short line about loading or a failure, shown in the header. Nil or empty means nothing to report.
	Status() *ui.Status

	Init() tea.Cmd
	Update(tea.Msg) (Model, tea.Cmd)
	View() string
}
