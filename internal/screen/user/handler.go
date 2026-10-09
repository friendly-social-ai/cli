package user

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/friendly-social-ai/cli/internal/browser"
	"github.com/friendly-social-ai/cli/internal/keys"
	"github.com/friendly-social-ai/cli/internal/router"
	"github.com/friendly-social-ai/cli/internal/screen"
	"github.com/friendly-social-ai/cli/internal/screen/auth"
	"github.com/friendly-social-ai/cli/internal/ui"
	sdk "github.com/friendly-social-ai/golang-sdk"
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
	// loadedMsg carries the profile of person. The request wraps it into router.TargetMsg so it reaches this screen.
	// notice tells what a finished request did.
	loadedMsg struct {
		person  sdk.UserId
		profile *sdk.UserProfile
		err     error
		notice  string
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
	// sending hides connect and remove while one of them runs
	sending bool

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
	back := ui.Action{Key: keys.Bind("back", keys.Navigation.Back, keys.Common.Cancel), Msg: backMsg{}}
	if s.confirmRemove {
		return []ui.Action{
			{Key: keys.Bind("confirm remove", keys.User.Remove), Msg: removeMsg{}},
			{Key: keys.Bind("cancel", keys.Common.Cancel), Msg: cancelRemoveMsg{}},
		}
	}

	if s.profile == nil {
		return []ui.Action{back}
	}

	var actions []ui.Action
	if !s.sending {
		switch s.profile.User.Friendship {
		case sdk.FriendshipFriends:
			actions = append(actions, ui.Action{Key: keys.Bind("remove friend", keys.User.Remove), Msg: removeMsg{}})
		case sdk.FriendshipIncomingRequest:
			actions = append(actions, ui.Action{Key: keys.Bind("accept", keys.User.Connect), Msg: connectMsg{}})
		case sdk.FriendshipOutgoingRequest:
		default:
			actions = append(actions, ui.Action{Key: keys.Bind("connect", keys.User.Connect), Msg: connectMsg{}})
		}
	}

	if s.profile.User.SocialLink.Value() != "" {
		actions = append(actions, ui.Action{Key: keys.Bind("open social link", keys.User.OpenSocial), Msg: openSocialMsg{}})
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
		return router.TargetMsg{Type: screen.TypeUser, Inner: loadedMsg{person: person.Id, profile: profile, err: err}}
	}
}

// request runs fn for the shown person, then reloads the profile to show the new friendship and shows notice.
func (s Screen) request(status, notice string, fn func(*sdk.Authorization, sdk.UserDetails) error) tea.Cmd {
	user, person := s.user, s.person
	s.content.status.Busy(status)
	return func() tea.Msg {
		if err := fn(user, person); err != nil {
			return router.TargetMsg{Type: screen.TypeUser, Inner: loadedMsg{person: person.Id, err: err}}
		}

		profile, err := s.service.get(user, person)
		return router.TargetMsg{Type: screen.TypeUser, Inner: loadedMsg{person: person.Id, profile: profile, err: err, notice: notice}}
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
		s.person, s.from, s.profile, s.confirmRemove, s.sending = msg.Person, msg.From, nil, false, false
		return s, s.load()
	case loadedMsg:
		// drop a response for a person left before it arrived
		if msg.person != s.person.Id {
			return s, nil
		}

		s.sending = false
		if msg.err != nil {
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
			return s, nil
		}

		s.profile = msg.profile
		s.content.status.Set("")
		if msg.notice != "" {
			return s, s.content.status.Notice(msg.notice)
		}
	case connectMsg:
		notice := "request sent"
		if s.profile.User.Friendship == sdk.FriendshipIncomingRequest {
			notice = "accepted"
		}

		s.sending = true
		return s, s.request("sending request", notice, s.service.connect)
	case removeMsg:
		if !s.confirmRemove {
			s.confirmRemove = true
			return s, nil
		}

		s.confirmRemove, s.sending = false, true
		return s, s.request("removing", "removed from friends", s.service.remove)
	case cancelRemoveMsg:
		s.confirmRemove = false
	case openSocialMsg:
		link, person := s.profile.User.SocialLink.Value(), s.person.Id
		return s, func() tea.Msg {
			if err := browser.Open(link); err != nil {
				return router.TargetMsg{Type: screen.TypeUser, Inner: loadedMsg{person: person, err: err}}
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

	view := ui.BoldStyle.Render(details.Nickname.Value()) + "\n" + ui.Fields(
		"friendship", friendship,
		"description", details.Description.Value(),
		"interests", strings.Join(interests, ", "),
		"social link", details.SocialLink.Value(),
		"common friends", strings.Join(common, ", "))
	if s.confirmRemove {
		view += "\n\n" + ui.DangerStyle.Render("Remove "+details.Nickname.Value()+" from friends? Press "+keys.User.Remove.Key()+" again to confirm.")
	}

	// long descriptions wrap to the window instead of running off its edge
	return lipgloss.NewStyle().Width(max(s.width, 20)).Render(view)
}
