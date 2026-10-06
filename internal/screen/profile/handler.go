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
	toggleEmailMsg  struct{}
	// loadedMsg carries profile of the logged in user. The request wraps it into router.TargetMsg so it reaches this screen.
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

	// self is the loaded profile, nil until it loads
	self *sdk.UserDetails
	// showEmail reveals bound email, which is masked by default
	showEmail bool

	content struct {
		// status shows loading, errors and logged out state when there is no profile to show
		status *ui.Label
	}
}

// New creates new Screen from Service.
func New(service *Service) Screen {
	result := Screen{
		service: service,
	}

	result.content.status = ui.NewLabel(ui.MutedStyle.Render("log in to see your profile"))
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
		var actions []ui.Action
		if s.email() != "" {
			desc := "show email"
			if s.showEmail {
				desc = "hide email"
			}

			actions = append(actions, ui.Action{Key: ui.Key("e", desc), Msg: toggleEmailMsg{}})
		}

		return append(actions,
			ui.Action{Key: ui.Key("x", "logout"), Msg: logoutMsg{}},
			ui.Action{Key: ui.Key("esc", "back"), Msg: screen.ChangeMsg{NewType: screen.TypeHome}})
	}

	return []ui.Action{{Key: ui.Key("esc", "back"), Msg: screen.ChangeMsg{NewType: screen.TypeHome}}}
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
}

// email returns email bound to the account, empty when there is none or the profile didn't load.
func (s Screen) email() string {
	if s.self == nil || s.self.Email == nil {
		return ""
	}

	return s.self.Email.Value()
}

// shownEmail returns email as displayed, masked unless revealed.
func (s Screen) shownEmail() string {
	if s.showEmail {
		return s.email()
	}

	return "***"
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
			s.content.status.Set(ui.DangerStyle.Render(err.Error()))
			return s, nil
		}

		return s, tea.Batch(
			screen.Send(router.BroadcastMsg{Inner: auth.LogoutMsg{}}),
			screen.Send(screen.ChangeMsg{NewType: screen.TypeHome}))
	case cancelLogoutMsg:
		s.confirmLogout = false
		return s, nil
	case toggleEmailMsg:
		s.showEmail = !s.showEmail
		return s, nil
	case auth.LogoutMsg:
		s.loggedIn, s.self, s.showEmail = false, nil, false
		s.content.status.Set(ui.MutedStyle.Render("log in to see your profile"))
		return s, nil
	case auth.LoginMsg:
		s.loggedIn, s.self, s.showEmail = true, nil, false
		s.content.status.Set(ui.MutedStyle.Render("loading..."))
		return s, func() tea.Msg {
			self, err := s.service.get(msg.User)
			return router.TargetMsg{Type: screen.TypeProfile, Inner: loadedMsg{self: self, err: err}}
		}
	case loadedMsg:
		if msg.err != nil {
			s.content.status.Set(ui.DangerStyle.Render(fmt.Sprintf("error loading profile: %s", msg.err.Error())))
			return s, nil
		}

		s.self = msg.self
		s.content.status.Set("")
		return s, nil
	}

	return s, nil
}

// profile renders the loaded profile, empty until it loads.
func (s Screen) profile() string {
	if s.self == nil {
		return ""
	}

	interests := make([]string, len(s.self.Interests.Value()))
	for i, interest := range s.self.Interests.Value() {
		interests[i] = interest.Value()
	}

	email := ""
	if s.email() != "" {
		email = s.shownEmail()
	}

	return ui.BoldStyle.Render(s.self.Nickname.Value()) + "\n" + ui.Fields(
		"email", email,
		"description", s.self.Description.Value(),
		"interests", strings.Join(interests, ", "),
		"social link", s.self.SocialLink.Value())
}

func (s Screen) View() string {
	var parts []string
	for _, part := range []string{s.content.status.View(), s.profile()} {
		if part != "" {
			parts = append(parts, part)
		}
	}

	if s.confirmLogout {
		// like the web, warn harder when there is no email to log back in with
		warning := "You have no email bound. Logging out loses this account for good. Log out anyway?"
		switch {
		case s.email() != "" && s.showEmail:
			warning = "Log out? You can log back in with " + s.email() + "."
		case s.email() != "":
			warning = "Log out? You can log back in with your email."
		}

		parts = append(parts, ui.DangerStyle.Render(warning))
	}

	return strings.Join(parts, "\n\n")
}
