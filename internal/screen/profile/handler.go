package profile

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/friendly-social-ai/cli/internal/keys"
	"github.com/friendly-social-ai/cli/internal/router"
	"github.com/friendly-social-ai/cli/internal/screen"
	"github.com/friendly-social-ai/cli/internal/screen/auth"
	"github.com/friendly-social-ai/cli/internal/ui"
	sdk "github.com/friendly-social-ai/golang-sdk"
)

// Messages produced by key actions of the screen.
type (
	logoutMsg       struct{}
	cancelLogoutMsg struct{}
	toggleEmailMsg  struct{}
	editMsg         struct{}
	cancelEditMsg   struct{}
	saveMsg         struct{}
	// fieldMsg moves typing by step fields, down for a positive step and up for a negative one.
	fieldMsg struct{ step int }
	// loadedMsg carries profile of the logged in user. The request wraps it into router.TargetMsg so it reaches this screen.
	loadedMsg struct {
		self *sdk.UserDetails
		err  error
	}
	// savedMsg carries the profile reloaded after saving, or the error of saving
	savedMsg struct {
		self *sdk.UserDetails
		err  error
	}
)

// Screen is a model of profile screen.
type Screen struct {
	service       *Service
	user          *sdk.Authorization
	confirmLogout bool
	// editing shows the edit form in place of the profile
	editing bool

	// self is the loaded profile, nil until it loads
	self *sdk.UserDetails
	// showEmail reveals bound email, which is masked by default
	showEmail bool

	content struct {
		status *ui.Status

		list   *ui.List
		fields []*ui.Field
		field  struct {
			nickname    *ui.Field
			description *ui.Field
			interests   *ui.Field
			social      *ui.Field
		}
	}

	width int
}

// field returns input labeled with its prompt, since its value hides a placeholder.
func field(label string, limit int) *ui.Field {
	input := textinput.New()
	input.Prompt = label + ": "
	input.CharLimit = limit
	styles := input.Styles()
	styles.Focused.Prompt = ui.MutedStyle
	styles.Blurred.Prompt = ui.MutedStyle
	input.SetStyles(styles)
	return ui.NewField(input)
}

// New creates new Screen from Service.
func New(service *Service) Screen {
	result := Screen{
		service: service,
	}

	result.content.status = ui.NewStatus()

	result.content.field.nickname = field("nickname", 256)
	result.content.field.description = field("description", 1024)
	result.content.field.interests = field("interests", 0)
	result.content.field.social = field("social link", 1024)
	result.content.fields = []*ui.Field{
		result.content.field.nickname,
		result.content.field.description,
		result.content.field.interests,
		result.content.field.social,
	}

	result.content.list = ui.NewList()
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
	case s.editing:
		return []ui.Action{
			{Key: keys.Bind("save", keys.Common.Confirm), Msg: saveMsg{}},
			{Key: keys.Bind("next field", keys.Common.NextField), Msg: fieldMsg{step: 1}},
			{Key: keys.Bind("previous field", keys.Common.PreviousField), Msg: fieldMsg{step: -1}},
			{Key: keys.Bind("cancel", keys.Common.Cancel), Msg: cancelEditMsg{}},
		}
	case s.confirmLogout:
		return []ui.Action{
			{Key: keys.Bind("confirm logout", keys.Profile.Logout), Msg: logoutMsg{}},
			{Key: keys.Bind("cancel", keys.Common.Cancel), Msg: cancelLogoutMsg{}},
		}
	case s.user != nil:
		var actions []ui.Action
		if s.self != nil {
			actions = append(actions, ui.Action{Key: keys.Bind("edit", keys.Profile.Edit), Msg: editMsg{}})
		}

		if s.email() != "" {
			desc := "show email"
			if s.showEmail {
				desc = "hide email"
			}

			actions = append(actions, ui.Action{Key: keys.Bind(desc, keys.Profile.ToggleEmail), Msg: toggleEmailMsg{}})
		}

		return append(actions, ui.Action{Key: keys.Bind("logout", keys.Profile.Logout), Msg: logoutMsg{}})
	}

	return nil
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
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(err)))
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
	case tea.WindowSizeMsg:
		s.width = msg.Width
		return s, nil
	case editMsg:
		interests := make([]string, len(s.self.Interests.Value()))
		for i, interest := range s.self.Interests.Value() {
			interests[i] = interest.Value()
		}

		s.content.field.nickname.Raw().SetValue(s.self.Nickname.Value())
		s.content.field.description.Raw().SetValue(s.self.Description.Value())
		s.content.field.interests.Raw().SetValue(strings.Join(interests, ", "))
		s.content.field.social.Raw().SetValue(s.self.SocialLink.Value())

		items := make([]ui.Component, len(s.content.fields))
		for i, field := range s.content.fields {
			items[i] = field
		}

		s.editing = true
		s.content.list.Reset(items...)
		return s, nil
	case cancelEditMsg:
		s.stopEditing()
		s.content.status.Set("")
		return s, nil
	case saveMsg:
		return s, s.save()
	case fieldMsg:
		return s, s.content.list.SelectFocused(s.content.list.Cursor() + msg.step)
	case savedMsg:
		if msg.err != nil {
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
			return s, nil
		}

		s.self = msg.self
		s.stopEditing()
		return s, s.content.status.Notice("profile saved")
	case auth.LogoutMsg:
		s.user, s.self, s.showEmail, s.editing = nil, nil, false, false
		s.content.status.Set("")
		return s, nil
	case auth.LoginMsg:
		s.user, s.self, s.showEmail, s.editing = msg.User, nil, false, false
		s.content.status.Busy("loading")
		return s, func() tea.Msg {
			self, err := s.service.get(msg.User)
			return router.TargetMsg{Type: screen.TypeProfile, Inner: loadedMsg{self: self, err: err}}
		}
	case loadedMsg:
		if msg.err != nil {
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
			return s, nil
		}

		s.self = msg.self
		s.content.status.Set("")
		return s, nil
	}

	if s.editing {
		_, cmd := s.content.list.Update(msg)
		return s, cmd
	}

	return s, nil
}

// stopEditing hides the edit form and unfocuses its field.
func (s *Screen) stopEditing() {
	s.editing = false
	s.content.list.Update(ui.UnfocusMsg{})
}

// save edits the profile with the form, then reloads it.
func (s Screen) save() tea.Cmd {
	user, self := s.user, s.self
	nickname, description := s.content.field.nickname.Value(), s.content.field.description.Value()
	interests, social := s.content.field.interests.Value(), s.content.field.social.Value()

	s.content.status.Busy("saving")
	return func() tea.Msg {
		if err := s.service.edit(user, self, nickname, description, interests, social); err != nil {
			return router.TargetMsg{Type: screen.TypeProfile, Inner: savedMsg{err: err}}
		}

		self, err := s.service.get(user)
		return router.TargetMsg{Type: screen.TypeProfile, Inner: savedMsg{self: self, err: err}}
	}
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

	// long descriptions wrap to the window instead of running off its edge
	return lipgloss.NewStyle().Width(max(s.width, 20)).Render(ui.BoldStyle.Render(s.self.Nickname.Value()) + "\n" + ui.Fields(
		"email", email,
		"description", ui.Emojize(s.self.Description.Value()),
		"interests", strings.Join(interests, ", "),
		"social link", s.self.SocialLink.Value()))
}

// Unsaved reports whether the profile is being edited.
func (s Screen) Unsaved() bool {
	return s.editing
}

// Typing reports whether the edit form takes typed text.
func (s Screen) Typing() bool {
	return s.editing
}

func (s Screen) Status() *ui.Status {
	return s.content.status
}

func (s Screen) View() string {
	if s.user == nil {
		return ui.MutedStyle.Render("log in to see your profile")
	}

	if s.editing {
		for _, field := range s.content.fields {
			field.Raw().SetWidth(max(s.width-10-lipgloss.Width(field.Raw().Prompt), 10))
		}

		return s.content.list.View()
	}

	var parts []string
	if profile := s.profile(); profile != "" {
		parts = append(parts, profile)
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

		parts = append(parts, ui.DangerStyle.Render(warning+" Press "+keys.Profile.Logout.Key()+" again to confirm."))
	}

	return strings.Join(parts, "\n\n")
}
