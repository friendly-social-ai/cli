package register

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/ui"
)

// Screen is a model of registration screen.
type Screen struct {
	service *Service

	content struct {
		list   *ui.List
		status *ui.Label

		fields []*ui.Field
		field  struct {
			nickname    *ui.Field
			description *ui.Field
			interests   *ui.Field
			social      *ui.Field
		}

		buttons []*ui.Button
		button  struct {
			submit *ui.Button
			back   *ui.Button
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

	result.content.field.nickname = field("Nickname", 256)
	result.content.field.description = field("Description", 1024)
	result.content.field.interests = field("Interests", 0)
	result.content.field.social = field("Social Link", 1024)

	result.content.button.submit = ui.NewButton("Submit",
		func() tea.Msg {
			result.content.status.Set("authenticating...")
			user, err := service.register(
				result.content.field.nickname.Value(),
				result.content.field.description.Value(),
				result.content.field.interests.Value(),
				result.content.field.social.Value())

			if err != nil {
				return screen.ErrorMsg{Value: err}
			}

			return router.BroadcastMsg{Inner: auth.LoginMsg{User: user}}
		})
	result.content.button.back = ui.NewButton("Back", func() tea.Msg {
		return screen.ChangeMsg{NewType: screen.TypeHome}
	})

	result.content.fields = []*ui.Field{
		result.content.field.nickname,
		result.content.field.description,
		result.content.field.interests,
		result.content.field.social,
	}

	result.content.buttons = []*ui.Button{
		result.content.button.submit,
		result.content.button.back,
	}

	result.content.status = ui.NewLabel("")
	result.content.list = ui.NewList(
		result.content.field.nickname,
		result.content.field.description,
		result.content.field.interests,
		result.content.field.social,
		result.content.button.submit,
		result.content.button.back)

	return result
}

func (Screen) ID() screen.Type {
	return screen.TypeRegister
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
		s.height = msg.Height
	case auth.LoginMsg:
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
