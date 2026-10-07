package user

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/friendly-social/cli/internal/browser"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
)

// OpenMsg asks user screen to show the profile of person. From is the screen to return to.
type OpenMsg struct {
	Person sdk.UserDetails
	From   screen.Type
}

// Messages produced by key actions of the screen.
type (
	connectMsg      struct{}
	removeMsg       struct{}
	cancelRemoveMsg struct{}
	openSocialMsg   struct{}
	backMsg         struct{}
	// loadedMsg carries the profile of the shown person. The request wraps it into router.TargetMsg so it reaches
	// this screen.
	loadedMsg struct {
		profile *sdk.UserProfile
		err     error
	}
)

// Screen is a model of user screen, which shows the profile of another user.
type Screen struct {
	service *Service
	user    *sdk.Authorization

	person  sdk.UserDetails
	profile *sdk.UserProfile
	from    screen.Type

	// confirmRemove asks to press the key again before ending the friendship
	confirmRemove bool

	content struct {
		status *ui.Status
	}

	width int
}

// New creates new Screen from Service.
func New(service *Service) Screen {
	result := Screen{
		service: service,
	}

	result.content.status = ui.NewStatus()
	return result
}

func (Screen) ID() screen.Type {
	return screen.TypeUser
}

func (Screen) Init() tea.Cmd {
	return nil
}

func (s Screen) actions() []ui.Action {
	back := ui.Action{Key: ui.Key("h", "back"), Msg: backMsg{}}
	if s.confirmRemove {
		return []ui.Action{
			{Key: ui.Key("x", "confirm remove"), Msg: removeMsg{}},
			{Key: ui.Key("esc", "cancel"), Msg: cancelRemoveMsg{}},
		}
	}

	if s.profile == nil {
		return []ui.Action{back}
	}

	var actions []ui.Action
	switch s.profile.User.Friendship {
	case sdk.FriendshipFriends:
		actions = append(actions, ui.Action{Key: ui.Key("x", "remove friend"), Msg: removeMsg{}})
	case sdk.FriendshipIncomingRequest:
		actions = append(actions, ui.Action{Key: ui.Key("a", "accept"), Msg: connectMsg{}})
	case sdk.FriendshipOutgoingRequest:
	default:
		actions = append(actions, ui.Action{Key: ui.Key("a", "connect"), Msg: connectMsg{}})
	}

	if s.profile.User.SocialLink.Value() != "" {
		actions = append(actions, ui.Action{Key: ui.Key("o", "open social link"), Msg: openSocialMsg{}})
	}

	return append(actions, back)
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
}

func (s Screen) load() tea.Cmd {
	user, person := s.user, s.person
	s.content.status.Busy("loading")
	return func() tea.Msg {
		profile, err := s.service.get(user, person)
		return router.TargetMsg{Type: screen.TypeUser, Inner: loadedMsg{profile: profile, err: err}}
	}
}

// request runs fn for the shown person, then reloads the profile to show the new friendship.
func (s Screen) request(status string, fn func(*sdk.Authorization, sdk.UserDetails) error) tea.Cmd {
	user, person := s.user, s.person
	s.content.status.Busy(status)
	return func() tea.Msg {
		if err := fn(user, person); err != nil {
			return router.TargetMsg{Type: screen.TypeUser, Inner: loadedMsg{err: err}}
		}

		profile, err := s.service.get(user, person)
		return router.TargetMsg{Type: screen.TypeUser, Inner: loadedMsg{profile: profile, err: err}}
	}
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.ActionMsg:
		action := ui.Dispatch(s.actions(), msg)
		// any other key drops pending remove confirmation
		if _, ok := action.(removeMsg); !ok {
			s.confirmRemove = false
		}

		if action != nil {
			return s, screen.Send(action)
		}
	case tea.WindowSizeMsg:
		s.width = msg.Width
	case auth.LoginMsg:
		s.user = msg.User
	case auth.LogoutMsg:
		s.user, s.profile, s.confirmRemove = nil, nil, false
		s.content.status.Set("")
	case OpenMsg:
		s.person, s.from, s.profile, s.confirmRemove = msg.Person, msg.From, nil, false
		return s, s.load()
	case loadedMsg:
		if msg.err != nil {
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
			return s, nil
		}

		s.profile = msg.profile
		s.content.status.Set("")
	case connectMsg:
		return s, s.request("sending request", s.service.connect)
	case removeMsg:
		if !s.confirmRemove {
			s.confirmRemove = true
			return s, nil
		}

		s.confirmRemove = false
		return s, s.request("removing", s.service.remove)
	case cancelRemoveMsg:
		s.confirmRemove = false
	case openSocialMsg:
		link := s.profile.User.SocialLink.Value()
		return s, func() tea.Msg {
			if err := browser.Open(link); err != nil {
				return router.TargetMsg{Type: screen.TypeUser, Inner: loadedMsg{err: err}}
			}

			return nil
		}
	case backMsg:
		return s, screen.Send(screen.ChangeMsg{NewType: s.from})
	}

	return s, nil
}

func (s Screen) Status() *ui.Status {
	return s.content.status
}

func (s Screen) View() string {
	if s.profile == nil {
		return ui.BoldStyle.Render(s.person.Nickname.Value())
	}

	details := s.profile.User
	interests := make([]string, len(details.Interests.Value()))
	for i, interest := range details.Interests.Value() {
		interests[i] = interest.Value()
	}

	common := make([]string, len(s.profile.CommonFriends))
	for i, friend := range s.profile.CommonFriends {
		common[i] = friend.Nickname.Value()
	}

	friendship := map[sdk.Friendship]string{
		sdk.FriendshipFriends:         "friends",
		sdk.FriendshipIncomingRequest: "wants to connect with you",
		sdk.FriendshipOutgoingRequest: "request sent",
	}[details.Friendship]

	// long descriptions wrap to the window instead of running off its edge
	return lipgloss.NewStyle().Width(max(s.width, 20)).Render(ui.BoldStyle.Render(details.Nickname.Value()) + "\n" + ui.Fields(
		"friendship", friendship,
		"description", details.Description.Value(),
		"interests", strings.Join(interests, ", "),
		"social link", details.SocialLink.Value(),
		"common friends", strings.Join(common, ", ")))
}
