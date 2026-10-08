package home

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/friendly-social-ai/cli/internal/router"
	"github.com/friendly-social-ai/cli/internal/screen"
	"github.com/friendly-social-ai/cli/internal/screen/auth"
	"github.com/friendly-social-ai/cli/internal/ui"
)

// Screen is a model of home screen, the menu of a logged out user. After login the tabs replace it.
type Screen struct {
	// checked tells that the saved login was checked. The menu stays empty until then.
	checked bool
	width   int

	content struct {
		list *ui.List
	}
}

// New returns new initial model of home screen.
func New() Screen {
	result := Screen{}

	result.content.list = ui.NewList(
		ui.NewButton("Login", func() tea.Msg {
			return screen.ChangeMsg{NewType: screen.TypeAuth}
		}),
		ui.NewButton("Register", func() tea.Msg {
			return screen.ChangeMsg{NewType: screen.TypeRegister}
		}))

	return result
}

func (Screen) ID() screen.Type {
	return screen.TypeHome
}

func (s Screen) Keys() []key.Binding {
	if !s.checked {
		return nil
	}

	return []key.Binding{ui.Key("l", "open", "enter")}
}

func (s Screen) Init() tea.Cmd {
	return func() tea.Msg {
		return router.TargetMsg{Type: s.ID(), Inner: ui.SelectMsg{}}
	}
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		return s, nil
	case auth.LoginMsg, auth.LogoutMsg:
		s.checked = true
		s.content.list.Select(0)
		return s, nil
	}

	_, cmd := s.content.list.Update(msg)
	return s, cmd
}

func (Screen) Status() *ui.Status {
	return nil
}

func (s Screen) View() string {
	if !s.checked {
		return ""
	}

	s.content.list.SetWidth(s.width)
	return s.content.list.View()
}
