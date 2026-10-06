package profile

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
)

// Messages produced by key actions of the screen.
type (
	logoutMsg       struct{}
	cancelLogoutMsg struct{}
	// loadedMsg carries profile of the logged in user, wrapped into router.TargetMsg so it reaches this screen.
	loadedMsg struct {
		self *sdk.UserDetails
		err  error
	}
)

// Screen is a model of profile screen.
type Screen struct {
	service       *Service
	loggedIn      bool
	confirmLogout bool

	// email bound to the account, empty when there is none or the profile didn't load
	email string

	content struct {
		label *ui.Label
	}
}

// New creates new Screen from Service.
func New(service *Service) Screen {
	result := Screen{
		service: service,
	}

	result.content.label = ui.NewLabel(ui.MutedStyle.Render("log in to see your profile"))
	return result
}

func (Screen) ID() screen.Type {
	return screen.TypeProfile
}

func (Screen) Init() tea.Cmd {
	return nil
}

func (s Screen) actions() []ui.Action {
	switch {
	case s.confirmLogout:
		return []ui.Action{
			{Key: ui.Key("x", "confirm logout"), Msg: logoutMsg{}},
			{Key: ui.Key("esc", "cancel"), Msg: cancelLogoutMsg{}},
		}
	case s.loggedIn:
		return []ui.Action{
			{Key: ui.Key("x", "logout"), Msg: logoutMsg{}},
			{Key: ui.Key("esc", "back"), Msg: screen.ChangeMsg{NewType: screen.TypeHome}},
		}
	}

	return []ui.Action{{Key: ui.Key("esc", "back"), Msg: screen.ChangeMsg{NewType: screen.TypeHome}}}
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.ActionMsg:
		action := ui.Dispatch(s.actions(), msg)
		// any other key drops pending logout confirmation
		if _, ok := action.(logoutMsg); !ok {
			s.confirmLogout = false
		}

		if action != nil {
			return s, screen.Send(action)
		}
	case logoutMsg:
		if !s.confirmLogout {
			s.confirmLogout = true
			return s, nil
		}

		s.confirmLogout = false
		if err := auth.Clear(); err != nil {
			s.content.label.Set(ui.DangerStyle.Render(err.Error()))
			return s, nil
		}

		return s, tea.Batch(
			screen.Send(router.BroadcastMsg{Inner: auth.LogoutMsg{}}),
			screen.Send(screen.ChangeMsg{NewType: screen.TypeHome}))
	case cancelLogoutMsg:
		s.confirmLogout = false
		return s, nil
	case auth.LogoutMsg:
		s.loggedIn = false
		s.email = ""
		s.content.label.Set(ui.MutedStyle.Render("log in to see your profile"))
		return s, nil
	case auth.LoginMsg:
		s.loggedIn = true
		s.email = ""
		s.content.label.Set(ui.MutedStyle.Render("loading..."))
		return s, func() tea.Msg {
			self, err := s.service.get(msg.User)
			return router.TargetMsg{Type: screen.TypeProfile, Inner: loadedMsg{self: self, err: err}}
		}
	case loadedMsg:
		if msg.err != nil {
			s.content.label.Set(ui.DangerStyle.Render(fmt.Sprintf("error loading profile: %s", msg.err.Error())))
			return s, nil
		}

		self := msg.self
		if self.Email != nil {
			s.email = self.Email.Value()
		}

		interests := make([]string, len(self.Interests.Value()))
		for i, interest := range self.Interests.Value() {
			interests[i] = interest.Value()
		}

		s.content.label.Set(ui.BoldStyle.Render(self.Nickname.Value()) + "\n" + ui.Fields(
			"email", s.email,
			"description", self.Description.Value(),
			"interests", strings.Join(interests, ", "),
			"social link", self.SocialLink.Value()))
		return s, nil
	}

	return s, nil
}

func (s Screen) View() string {
	if s.confirmLogout {
		// same as web: warn harder when there is no email to log back in with
		warning := "You have no email bound. Logging out loses this account for good. Log out anyway?"
		if s.email != "" {
			warning = "Log out? You can log back in with " + s.email + "."
		}

		return s.content.label.View() + "\n\n" + ui.DangerStyle.Render(warning)
	}

	return s.content.label.View()
}
