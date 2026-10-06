package home

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/activity"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/ui"
)

// Screen is a model of home screen.
type Screen struct {
	loggedIn bool

	content struct {
		list *ui.List

		buttons struct {
			community *ui.Button
			activity  *ui.Button
			people    *ui.Button
			profile   *ui.Button
			login     *ui.Button
			register  *ui.Button
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
	result.content.buttons.activity = ui.NewButton("Activity", func() tea.Msg {
		return screen.ChangeMsg{NewType: screen.TypeActivity}
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

	result.content.list = ui.NewList()
	result.content.list.Reset(result.items()...)

	return result
}

// items builds menu for the current login state.
func (s Screen) items() []tea.Model {
	if s.loggedIn {
		return []tea.Model{s.content.buttons.community, s.content.buttons.activity, s.content.buttons.people, s.content.buttons.profile}
	}

	return []tea.Model{s.content.buttons.login, s.content.buttons.register}
}

func (Screen) ID() screen.Type {
	return screen.TypeHome
}

func (Screen) Keys() []key.Binding {
	return []key.Binding{ui.Key("enter", "open")}
}

func (s Screen) Init() tea.Cmd {
	return func() tea.Msg {
		return router.TargetMsg{Type: s.ID(), Inner: ui.SelectMsg{}}
	}
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case activity.UnreadMsg:
		title := "Activity"
		if msg.Count > 0 {
			title += ui.MutedStyle.Render(fmt.Sprintf(" · %d new", msg.Count))
		}

		s.content.buttons.activity.SetTitle(title)
		return s, nil
	case auth.LoginMsg:
		s.loggedIn = true
		s.content.list.Reset(s.items()...)
		return s, nil
	case auth.LogoutMsg:
		s.loggedIn = false
		s.content.buttons.activity.SetTitle("Activity")
		s.content.list.Reset(s.items()...)
		return s, nil
	}

	_, cmd := s.content.list.Update(msg)
	return s, cmd
}

func (s Screen) View() string {
	return s.content.list.View()
}
