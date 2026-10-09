package register

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/friendly-social-ai/cli/internal/keys"
	"github.com/friendly-social-ai/cli/internal/router"
	"github.com/friendly-social-ai/cli/internal/screen"
	"github.com/friendly-social-ai/cli/internal/screen/auth"
	"github.com/friendly-social-ai/cli/internal/ui"
)

// Messages produced by key actions of the screen.
type (
	// nextMsg moves to the next field, and submits from the last one
	nextMsg struct{}
	// fieldMsg moves typing by step fields, down for a positive step and up for a negative one.
	fieldMsg struct{ step int }
)

// Screen is a model of registration screen.
type Screen struct {
	service *Service
	// submitting locks the form until registration ends
	submitting bool

	content struct {
		list   *ui.List
		status *ui.Status

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
	result.content.field.interests = field("Interests (optional)", 0)
	result.content.field.social = field("Social Link (optional)", 1024)

	result.content.fields = []*ui.Field{
		result.content.field.nickname,
		result.content.field.description,
		result.content.field.interests,
		result.content.field.social,
	}

	result.content.status = ui.NewStatus()
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

func (s Screen) actions() []ui.Action {
	if s.submitting {
		return nil
	}

	next := ui.Action{Key: keys.Bind("next", keys.Common.Confirm), Msg: nextMsg{}}
	if s.content.list.Cursor() == len(s.content.fields)-1 {
		next = ui.Action{Key: keys.Bind("submit", keys.Common.Confirm), Msg: nextMsg{}}
	}

	return []ui.Action{
		next,
		{Key: keys.Bind("next field", keys.Common.NextField), Msg: fieldMsg{step: 1}},
		{Key: keys.Bind("previous field", keys.Common.PreviousField), Msg: fieldMsg{step: -1}},
		{Key: keys.Bind("back", keys.Common.Cancel), Msg: screen.ChangeMsg{NewType: screen.TypeHome}},
	}
}

// Typing reports that the screen always takes typed text.
func (Screen) Typing() bool {
	return true
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
}

func (s Screen) submit() tea.Cmd {
	nickname := s.content.field.nickname.Value()
	description := s.content.field.description.Value()
	interests := s.content.field.interests.Value()
	social := s.content.field.social.Value()

	s.content.status.Busy("authenticating")
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
	case nextMsg:
		if cursor := s.content.list.Cursor(); cursor < len(s.content.fields)-1 {
			return s, s.content.list.SelectFocused(cursor + 1)
		}

		s.submitting = true
		s.content.list.Update(ui.UnfocusMsg{})
		return s, s.submit()
	case fieldMsg:
		return s, s.content.list.SelectFocused(s.content.list.Cursor() + msg.step)
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
		s.submitting = false
		return s, func() tea.Msg {
			return screen.ChangeMsg{NewType: screen.TypeCommunity}
		}
	case screen.ErrorMsg:
		s.submitting = false
		s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.Value)))
		// typing goes back to the field it left
		return s, s.content.list.SelectFocused(s.content.list.Cursor())
	}

	if s.submitting {
		return s, nil
	}

	_, cmd := s.content.list.Update(msg)
	return s, cmd
}

// Unsaved reports whether any field has text.
func (s Screen) Unsaved() bool {
	return ui.Filled(s.content.fields)
}

func (s Screen) Status() *ui.Status {
	return s.content.status
}

func (s Screen) View() string {
	for _, field := range s.content.fields {
		field.Raw().SetWidth(s.width - 10)
	}

	return s.content.list.View()
}
