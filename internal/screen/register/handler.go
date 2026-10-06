package register

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/ui"
)

// submitMsg asks registration screen to register with filled fields.
type submitMsg struct{}

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
	result.content.field.social = field("Social Link (optional)", 1024)

	result.content.fields = []*ui.Field{
		result.content.field.nickname,
		result.content.field.description,
		result.content.field.interests,
		result.content.field.social,
	}

	result.content.status = ui.NewLabel("")
	result.content.list = ui.NewList(
		result.content.field.nickname,
		result.content.field.description,
		result.content.field.interests,
		result.content.field.social)

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

func (Screen) actions() []ui.Action {
	return []ui.Action{
		{Key: ui.Key("i", "type")},
		{Key: ui.Key("s", "submit"), Msg: submitMsg{}},
		{Key: ui.Key("esc", "back"), Msg: screen.ChangeMsg{NewType: screen.TypeHome}},
	}
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
}

func (s Screen) submit() tea.Cmd {
	nickname := s.content.field.nickname.Value()
	description := s.content.field.description.Value()
	interests := s.content.field.interests.Value()
	social := s.content.field.social.Value()

	s.content.status.Set(ui.MutedStyle.Render("authenticating..."))
	return func() tea.Msg {
		user, err := s.service.register(nickname, description, interests, social)
		if err != nil {
			return screen.ErrorMsg{Value: err}
		}

		return router.BroadcastMsg{Inner: auth.LoginMsg{User: user}}
	}
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.ActionMsg:
		if action := ui.Dispatch(s.actions(), msg); action != nil {
			return s, screen.Send(action)
		}
	case submitMsg:
		return s, s.submit()
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	case auth.LogoutMsg:
		for _, field := range s.content.fields {
			field.Raw().SetValue("")
		}

		s.content.status.Set("")
		return s, nil
	case auth.LoginMsg:
		return s, func() tea.Msg {
			return screen.ChangeMsg{NewType: screen.TypeHome}
		}
	case screen.ErrorMsg:
		s.content.status.Set(ui.DangerStyle.Render(msg.Value.Error()))
		return s, nil
	}

	_, cmd := s.content.list.Update(msg)
	return s, cmd
}

func (s Screen) View() string {
	for _, field := range s.content.fields {
		field.Raw().SetWidth(s.width - 10)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		s.content.list.View(),
		"",
		s.content.status.View(),
	)
}
