package auth

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
)

// LoginMsg signalizes that user logged in with new credentials.
type LoginMsg struct {
	User *sdk.Authorization
}

// Screen is a model of e-mail login screen.
type Screen struct {
	service *Service

	content struct {
		list   *ui.List
		status *ui.Label

		fields []*ui.Field
		field  struct {
			email *ui.Field
			code  *ui.Field
		}

		button struct {
			send    *ui.Button
			confirm *ui.Button
			back    *ui.Button
		}
	}

	width  int
	height int
}

func field(label string, limit int) *ui.Field {
	field := textinput.New()
	field.Placeholder = label
	field.CharLimit = limit
	field.Prompt = ""
	return ui.NewField(field)
}

// New creates new Screen from Service.
func New(service *Service) Screen {
	result := Screen{
		service: service,
	}

	result.content.field.email = field("E-mail", 2048)
	result.content.field.code = field("Code", 8)

	result.content.button.send = ui.NewButton("Send code",
		func() tea.Msg {
			result.content.status.Set("sending code...")
			email := result.content.field.email.Value()
			err := service.send(email)
			if err != nil {
				return screen.ErrorMsg{Value: err}
			}

			result.content.status.Set(fmt.Sprintf("code sent to %s, check your inbox", email))
			return screen.TickMsg{}
		})
	result.content.button.confirm = ui.NewButton("Confirm",
		func() tea.Msg {
			result.content.status.Set("authenticating...")
			user, err := service.confirm(
				result.content.field.email.Value(),
				result.content.field.code.Value())

			if err != nil {
				return screen.ErrorMsg{Value: err}
			}

			return router.BroadcastMsg{Inner: LoginMsg{User: user}}
		})
	result.content.button.back = ui.NewButton("Back", func() tea.Msg {
		return screen.ChangeMsg{NewType: screen.TypeHome}
	})

	result.content.fields = []*ui.Field{
		result.content.field.email,
		result.content.field.code,
	}

	result.content.status = ui.NewLabel("")
	result.content.list = ui.NewList(
		result.content.field.email,
		result.content.button.send,
		result.content.field.code,
		result.content.button.confirm,
		result.content.button.back)

	return result
}

func (Screen) ID() screen.Type {
	return screen.TypeAuth
}

func (s Screen) Init() tea.Cmd {
	return tea.Sequence(
		func() tea.Msg {
			user, err := Load()
			if err != nil {
				return screen.ErrorMsg{Value: err}
			}

			if user == nil {
				return nil
			}

			return router.BroadcastMsg{Inner: LoginMsg{User: user}}
		},
		func() tea.Msg {
			return router.TargetMsg{Type: s.ID(), Inner: ui.SelectMsg{}}
		})
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	case LoginMsg:
		return s, func() tea.Msg {
			return screen.ChangeMsg{NewType: screen.TypeHome}
		}
	case screen.ErrorMsg:
		s.content.status.Set(msg.Value.Error())
		return s, nil
	}

	_, cmd := s.content.list.Update(msg)
	return s, cmd
}

func (s Screen) View() string {
	for _, field := range s.content.fields {
		field.Raw().Width = s.width - 10
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		s.content.list.View(),
		"",
		s.content.status.View(),
	)
}
