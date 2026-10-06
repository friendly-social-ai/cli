package home

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/ui"
)

// Screen is a model of home screen.
type Screen struct {
	content struct {
		list *ui.List

		buttons struct {
			community *ui.Button
			people    *ui.Button
			profile   *ui.Button
			login     *ui.Button
			register  *ui.Button
			exit      *ui.Button
		}
	}
}

// New returns new initial model of home screen.
func New() Screen {
	result := Screen{}

	result.content.buttons.register = ui.NewButton("Register", func() tea.Msg {
		return screen.ChangeMsg{NewType: screen.TypeRegister}
	})
	result.content.buttons.community = ui.NewButton("Community", func() tea.Msg {
		return screen.ChangeMsg{NewType: screen.TypeCommunity}
	})
	result.content.buttons.people = ui.NewButton("People", func() tea.Msg {
		return screen.ChangeMsg{NewType: screen.TypePeople}
	})
	result.content.buttons.profile = ui.NewButton("Profile", func() tea.Msg {
		return screen.ChangeMsg{NewType: screen.TypeProfile}
	})
	result.content.buttons.login = ui.NewButton("Login", func() tea.Msg {
		return screen.ChangeMsg{NewType: screen.TypeAuth}
	})
	result.content.buttons.exit = ui.NewButton("Exit", tea.Quit)

	result.content.list = ui.NewList(
		result.content.buttons.community,
		result.content.buttons.people,
		result.content.buttons.profile,
		result.content.buttons.login,
		result.content.buttons.register,
		result.content.buttons.exit)

	return result
}

func (Screen) ID() screen.Type {
	return screen.TypeHome
}

func (s Screen) Init() tea.Cmd {
	return func() tea.Msg {
		return router.TargetMsg{Type: s.ID(), Inner: ui.SelectMsg{}}
	}
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	_, cmd := s.content.list.Update(msg)
	return s, cmd
}

func (s Screen) View() string {
	return s.content.list.View()
}
