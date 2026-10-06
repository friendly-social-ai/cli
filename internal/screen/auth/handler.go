package auth

import (
	"fmt"
	"log"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
)

// LoginMsg signals that user logged in with new credentials.
type LoginMsg struct {
	User *sdk.Authorization
}

// LogoutMsg signals that user logged out and saved credentials are gone. Expired means the server rejected them.
type LogoutMsg struct {
	Expired bool
}

// Messages produced by key actions of the screen.
type (
	sendMsg    struct{}
	confirmMsg struct{}
	// nextMsg sends the code and moves to the code field from the e-mail field. From the code field it confirms.
	nextMsg struct{}
)

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

	result.content.fields = []*ui.Field{
		result.content.field.email,
		result.content.field.code,
	}

	result.content.status = ui.NewLabel("")
	result.content.list = ui.NewList(
		result.content.field.email,
		result.content.field.code)

	return result
}

func (Screen) ID() screen.Type {
	return screen.TypeAuth
}

func (s Screen) Init() tea.Cmd {
	return tea.Sequence(
		func() tea.Msg {
			// screens show nothing until one of these messages arrives. A save that fails to load counts as logged out.
			user, err := Load()
			if err != nil {
				log.Printf("error: %v", err)
			}

			if user == nil {
				return router.BroadcastMsg{Inner: LogoutMsg{}}
			}

			return router.BroadcastMsg{Inner: LoginMsg{User: user}}
		},
		func() tea.Msg {
			return router.TargetMsg{Type: s.ID(), Inner: ui.SelectMsg{}}
		})
}

func (s Screen) actions() []ui.Action {
	next := ui.Action{Key: ui.Key("enter", "send code"), Msg: nextMsg{}}
	if s.content.list.Cursor() == 1 {
		next = ui.Action{Key: ui.Key("enter", "confirm"), Msg: nextMsg{}}
	}

	return []ui.Action{
		{Key: ui.Key("i", "type")},
		next,
		{Key: ui.Key("s", "send code"), Msg: sendMsg{}},
		{Key: ui.Key("c", "confirm"), Msg: confirmMsg{}},
		{Key: ui.Key("esc", "back"), Msg: screen.ChangeMsg{NewType: screen.TypeHome}},
	}
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
}

func (s Screen) send() tea.Cmd {
	email := s.content.field.email.Value()
	s.content.status.Set(ui.MutedStyle.Render("sending code..."))
	return func() tea.Msg {
		if err := s.service.send(email); err != nil {
			return screen.ErrorMsg{Value: err}
		}

		s.content.status.Set(fmt.Sprintf("code sent to %s, check your inbox", email))
		return screen.TickMsg{}
	}
}

func (s Screen) confirm() tea.Cmd {
	email, code := s.content.field.email.Value(), s.content.field.code.Value()
	s.content.status.Set(ui.MutedStyle.Render("authenticating..."))
	return func() tea.Msg {
		user, err := s.service.confirm(email, code)
		if err != nil {
			return screen.ErrorMsg{Value: err}
		}

		return router.BroadcastMsg{Inner: LoginMsg{User: user}}
	}
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.ActionMsg:
		if action := ui.Dispatch(s.actions(), msg); action != nil {
			return s, screen.Send(action)
		}
	case sendMsg:
		return s, s.send()
	case confirmMsg:
		return s, s.confirm()
	case nextMsg:
		if s.content.list.Cursor() == 0 {
			return s, tea.Batch(s.send(), s.content.list.SelectFocused(1))
		}

		return s, tea.Batch(screen.Send(ui.NormalMsg{}), s.confirm())
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	case LogoutMsg:
		for _, field := range s.content.fields {
			field.Raw().SetValue("")
		}

		s.content.status.Set("")
		if msg.Expired {
			s.content.status.Set(ui.DangerStyle.Render("session expired, log in again"))
		}

		return s, nil
	case ExpiredMsg:
		if err := Clear(); err != nil {
			log.Printf("error: %v", err)
		}

		return s, tea.Batch(
			screen.Send(ui.NormalMsg{}),
			screen.Send(router.BroadcastMsg{Inner: LogoutMsg{Expired: true}}),
			screen.Send(screen.ChangeMsg{NewType: screen.TypeAuth}))
	case LoginMsg:
		return s, func() tea.Msg {
			return screen.ChangeMsg{NewType: screen.TypeCommunity}
		}
	case screen.ErrorMsg:
		s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.Value)))
		return s, nil
	}

	_, cmd := s.content.list.Update(msg)
	return s, cmd
}

func (s Screen) Status() string {
	return s.content.status.View()
}

func (s Screen) View() string {
	for _, field := range s.content.fields {
		field.Raw().SetWidth(s.width - 10)
	}

	return s.content.list.View()
}
