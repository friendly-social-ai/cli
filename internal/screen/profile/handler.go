package profile

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"git.getfriend.ly/ai/cli/internal/keys"
	"git.getfriend.ly/ai/cli/internal/router"
	"git.getfriend.ly/ai/cli/internal/screen"
	"git.getfriend.ly/ai/cli/internal/screen/auth"
	"git.getfriend.ly/ai/cli/internal/screen/user"
	"git.getfriend.ly/ai/cli/internal/ui"
	sdk "git.getfriend.ly/ai/golang-sdk"
)

// Messages produced by key actions of the screen.
type (
	logoutMsg       struct{}
	cancelLogoutMsg struct{}
	toggleEmailMsg  struct{}
	editMsg         struct{}
	cancelEditMsg   struct{}
	saveMsg         struct{}
	inviteMsg       struct{}
	addMsg          struct{}
	emailMsg        struct{}
	cancelPromptMsg struct{}
	submitMsg       struct{}
	unlinkMsg       struct{}
	cancelUnlinkMsg struct{}
	// fieldMsg moves typing by step fields, down for a positive step and up for a negative one.
	fieldMsg struct{ step int }
	// openFriendMsg opens the profile of the friend at index.
	openFriendMsg  struct{ index int }
	filterMsg      struct{}
	filterDoneMsg  struct{}
	clearFilterMsg struct{}
	// loadedMsg carries profile and friends of the logged in user. The request wraps it into router.TargetMsg so it
	// reaches this screen. friendsErr is set when only the friends failed to load.
	loadedMsg struct {
		self       *sdk.UserDetails
		friends    []sdk.UserDetails
		err        error
		friendsErr error
	}
	// friendsMsg carries the friends of user, reloaded when the screen shows again
	friendsMsg struct {
		user    *sdk.Authorization
		friends []sdk.UserDetails
	}
	// savedMsg carries the profile reloaded after saving, or the error of saving
	savedMsg struct {
		self *sdk.UserDetails
		err  error
	}
	// invitedMsg carries the invite link of user and when it expires, or the error of creating it
	invitedMsg struct {
		user       *sdk.Authorization
		link       string
		expiration time.Time
		err        error
	}
	// submittedMsg carries the result of submitting the field for prompt as user. err is nil on success, and self is
	// the profile reloaded after the code confirms an email.
	submittedMsg struct {
		user   *sdk.Authorization
		prompt prompt
		self   *sdk.UserDetails
		err    error
	}
	// unlinkedMsg carries the profile of user reloaded after unlinking the email, or the error of unlinking
	unlinkedMsg struct {
		user *sdk.Authorization
		self *sdk.UserDetails
		err  error
	}
)

// prompt is what the field under the profile asks for.
type prompt int

const (
	promptNone prompt = iota
	promptInvite
	promptEmail
	promptCode
)

// Screen is a model of profile screen.
type Screen struct {
	service       *Service
	user          *sdk.Authorization
	confirmLogout bool
	// editing shows the edit form in place of the profile. saving locks it until the save ends.
	editing bool
	saving  bool

	// self is the loaded profile, nil until it loads
	self *sdk.UserDetails
	// friends are listed under the profile, nil until they load
	friends []sdk.UserDetails
	// showEmail reveals bound email, which is masked by default
	showEmail bool
	// invite is the link others add the user by, empty until the user asks for it
	invite struct {
		link       string
		expiration time.Time
	}
	// prompt shows the field for an invite, an email or its code. sending locks the screen until the request of the
	// field or the unlink request ends.
	prompt        prompt
	sending       bool
	confirmUnlink bool

	content struct {
		status *ui.Status

		// friends lists the friends to open. It is separate from list, which holds the edit form. filter narrows it by
		// nickname.
		friends *ui.List
		filter  *ui.Filter

		list   *ui.List
		fields []*ui.Field
		field  struct {
			nickname    *ui.Field
			description *ui.Field
			interests   *ui.Field
			social      *ui.Field
		}

		// prompt is the field under the profile, labeled for each prompt by ask
		prompt *ui.Field
	}

	width  int
	height int
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

	result.content.prompt = field("", 0)

	result.content.list = ui.NewList()
	result.content.friends = ui.NewList()
	result.content.filter = ui.NewFilter()
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
	case s.saving, s.sending:
		return nil
	case s.prompt != promptNone:
		submit := map[prompt]string{promptInvite: "add", promptEmail: "send code", promptCode: "confirm"}[s.prompt]
		return []ui.Action{
			{Key: keys.Bind(submit, keys.Common.Confirm), Msg: submitMsg{}},
			{Key: keys.Bind("cancel", keys.Common.Cancel), Msg: cancelPromptMsg{}},
		}
	case s.editing:
		return []ui.Action{
			{Key: keys.Bind("save", keys.Common.Confirm), Msg: saveMsg{}},
			{Key: keys.Bind("next field", keys.Common.NextField), Msg: fieldMsg{step: 1}},
			{Key: keys.Bind("previous field", keys.Common.PreviousField), Msg: fieldMsg{step: -1}},
			{Key: keys.Bind("cancel", keys.Common.Cancel), Msg: cancelEditMsg{}},
		}
	case s.content.filter.Typing():
		return []ui.Action{
			{Key: keys.Bind("done", keys.Common.Confirm), Msg: filterDoneMsg{}},
			{Key: keys.Bind("clear", keys.Common.Cancel), Msg: clearFilterMsg{}},
		}
	case s.confirmLogout:
		return []ui.Action{
			{Key: keys.Bind("confirm logout", keys.Profile.Logout), Msg: logoutMsg{}},
			{Key: keys.Bind("cancel", keys.Common.Cancel), Msg: cancelLogoutMsg{}},
		}
	case s.confirmUnlink:
		return []ui.Action{
			{Key: keys.Bind("confirm unlink", keys.Profile.UnlinkEmail), Msg: unlinkMsg{}},
			{Key: keys.Bind("cancel", keys.Common.Cancel), Msg: cancelUnlinkMsg{}},
		}
	case s.user != nil:
		var actions []ui.Action
		if len(s.listed()) > 0 {
			actions = append(actions, ui.Action{Key: keys.Bind("open friend", keys.Navigation.Open)})
		}

		if len(s.friends) > 0 {
			actions = append(actions, ui.Action{Key: keys.Bind("filter", keys.Common.Filter), Msg: filterMsg{}})
		}

		if s.content.filter.Query() != "" {
			actions = append(actions, ui.Action{Key: keys.Bind("clear filter", keys.Common.Cancel), Msg: clearFilterMsg{}})
		}

		if s.self != nil {
			actions = append(actions, ui.Action{Key: keys.Bind("edit profile", keys.Profile.Edit), Msg: editMsg{}})
		}

		if s.email() != "" {
			desc := "show email"
			if s.showEmail {
				desc = "hide email"
			}

			actions = append(actions,
				ui.Action{Key: keys.Bind(desc, keys.Profile.ToggleEmail), Msg: toggleEmailMsg{}},
				ui.Action{Key: keys.Bind("change email", keys.Profile.LinkEmail), Msg: emailMsg{}},
				ui.Action{Key: keys.Bind("unlink email", keys.Profile.UnlinkEmail), Msg: unlinkMsg{}})
		} else if s.self != nil {
			actions = append(actions, ui.Action{Key: keys.Bind("link email", keys.Profile.LinkEmail), Msg: emailMsg{}})
		}

		actions = append(actions,
			ui.Action{Key: keys.Bind("invite", keys.Profile.Invite), Msg: inviteMsg{}},
			ui.Action{Key: keys.Bind("add friend", keys.Profile.AddFriend), Msg: addMsg{}})
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
		// any other key drops pending logout and unlink confirmation
		if _, ok := action.(logoutMsg); !ok {
			s.confirmLogout = false
		}

		if _, ok := action.(unlinkMsg); !ok {
			s.confirmUnlink = false
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
		s.height = msg.Height
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
		s.saving = true
		s.content.list.Update(ui.UnfocusMsg{})
		return s, s.save()
	case fieldMsg:
		return s, s.content.list.SelectFocused(s.content.list.Cursor() + msg.step)
	case savedMsg:
		s.saving = false
		if msg.err != nil {
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
			if !s.editing {
				return s, nil
			}

			// typing goes back to the field it left
			return s, s.content.list.SelectFocused(s.content.list.Cursor())
		}

		s.self = msg.self
		s.stopEditing()
		return s, s.content.status.Notice("profile saved")
	case inviteMsg:
		s.content.status.Busy("creating invite")
		current := s.user
		return s, func() tea.Msg {
			link, expiration, err := s.service.invite(current)
			return router.TargetMsg{Type: screen.TypeProfile, Inner: invitedMsg{user: current, link: link, expiration: expiration, err: err}}
		}
	case invitedMsg:
		// drop the invite of a user who logged out before it arrived
		if msg.user != s.user {
			return s, nil
		}

		if msg.err != nil {
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
			return s, nil
		}

		s.invite.link, s.invite.expiration = msg.link, msg.expiration
		return s, tea.Batch(tea.SetClipboard(msg.link), s.content.status.Notice("copied invite link"))
	case addMsg:
		return s, s.ask(promptInvite)
	case emailMsg:
		return s, s.ask(promptEmail)
	case cancelPromptMsg:
		s.stopPrompt()
		s.content.status.Set("")
		return s, nil
	case submitMsg:
		// the backend sends no code for the bound email, so there is nothing to confirm
		if s.prompt == promptEmail && strings.TrimSpace(s.content.prompt.Value()) == s.email() {
			s.stopPrompt()
			return s, s.content.status.Notice("email already linked")
		}

		s.sending = true
		s.content.status.Busy(map[prompt]string{promptInvite: "adding friend", promptEmail: "sending code", promptCode: "confirming"}[s.prompt])
		current, p, value := s.user, s.prompt, s.content.prompt.Value()
		return s, func() tea.Msg {
			result := submittedMsg{user: current, prompt: p}
			switch p {
			case promptInvite:
				result.err = s.service.add(current, value)
			case promptEmail:
				result.err = s.service.link(current, value)
			case promptCode:
				if result.err = s.service.confirm(current, value); result.err == nil {
					result.self, result.err = s.service.get(current)
				}
			}

			return router.TargetMsg{Type: screen.TypeProfile, Inner: result}
		}
	case submittedMsg:
		// drop the result for a user who logged out before it arrived
		if msg.user != s.user {
			return s, nil
		}

		s.sending = false
		if msg.err != nil {
			// the field stays open with its text, so the user can fix it or try again
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
			return s, nil
		}

		switch msg.prompt {
		case promptInvite:
			s.stopPrompt()
			return s, tea.Batch(s.content.status.Notice("added friend"), s.reloadFriends())
		case promptEmail:
			notice := s.content.status.Notice("code sent to " + strings.TrimSpace(s.content.prompt.Value()))
			return s, tea.Batch(s.ask(promptCode), notice)
		case promptCode:
			s.self = msg.self
			s.stopPrompt()
			return s, s.content.status.Notice("email linked")
		}
	case unlinkMsg:
		if !s.confirmUnlink {
			s.confirmUnlink = true
			return s, nil
		}

		s.confirmUnlink, s.sending = false, true
		s.content.status.Busy("unlinking email")
		current := s.user
		return s, func() tea.Msg {
			result := unlinkedMsg{user: current}
			if result.err = s.service.unlink(current); result.err == nil {
				result.self, result.err = s.service.get(current)
			}

			return router.TargetMsg{Type: screen.TypeProfile, Inner: result}
		}
	case cancelUnlinkMsg:
		s.confirmUnlink = false
		return s, nil
	case unlinkedMsg:
		// drop the result for a user who logged out before it arrived
		if msg.user != s.user {
			return s, nil
		}

		s.sending = false
		if msg.err != nil {
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
			return s, nil
		}

		s.self, s.showEmail = msg.self, false
		return s, s.content.status.Notice("email unlinked")
	case auth.LogoutMsg:
		s.user, s.self, s.friends, s.showEmail, s.editing = nil, nil, nil, false, false
		s.invite.link, s.invite.expiration = "", time.Time{}
		s.confirmUnlink = false
		s.stopPrompt()
		s.content.filter.Stop()
		s.content.filter.Clear()
		s.content.friends.Reset()
		s.content.status.Set("")
		return s, nil
	case auth.LoginMsg:
		s.user, s.self, s.showEmail, s.editing = msg.User, nil, false, false
		s.invite.link, s.invite.expiration = "", time.Time{}
		s.confirmUnlink = false
		s.stopPrompt()
		s.content.status.Busy("loading")
		return s, func() tea.Msg {
			self, err := s.service.get(msg.User)
			if err != nil {
				return router.TargetMsg{Type: screen.TypeProfile, Inner: loadedMsg{err: err}}
			}

			friends, friendsErr := s.service.friends(msg.User)
			return router.TargetMsg{Type: screen.TypeProfile, Inner: loadedMsg{self: self, friends: friends, friendsErr: friendsErr}}
		}
	case loadedMsg:
		if msg.err != nil {
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
			return s, nil
		}

		s.self, s.friends = msg.self, msg.friends
		s.content.friends.Reset(s.friendItems()...)
		s.content.status.Set("")
		if msg.friendsErr != nil {
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.friendsErr)))
		}

		return s, nil
	case screen.ShownMsg:
		return s, s.reloadFriends()
	case friendsMsg:
		// drop friends of a user who logged out before they arrived
		if msg.user != s.user {
			return s, nil
		}

		s.friends = msg.friends
		s.content.friends.Set(s.friendItems()...)
		return s, nil
	case openFriendMsg:
		s.confirmLogout, s.confirmUnlink = false, false
		// a reload can shrink the friends before the open arrives
		if msg.index >= len(s.friends) {
			return s, nil
		}

		return s, tea.Sequence(
			screen.Send(router.TargetMsg{Type: screen.TypeUser, Inner: user.OpenMsg{Person: s.friends[msg.index], From: screen.TypeProfile}}),
			screen.Send(screen.ChangeMsg{NewType: screen.TypeUser}))
	case filterMsg:
		s.content.filter.Start()
		return s, nil
	case filterDoneMsg:
		s.content.filter.Stop()
		return s, nil
	case clearFilterMsg:
		s.content.filter.Stop()
		s.content.filter.Clear()
		s.content.friends.Reset(s.friendItems()...)
		return s, nil
	}

	switch {
	case s.editing && !s.saving:
		_, cmd := s.content.list.Update(msg)
		return s, cmd
	case s.prompt != promptNone && !s.sending:
		_, cmd := s.content.prompt.Update(msg)
		return s, cmd
	case s.editing, s.prompt != promptNone:
		return s, nil
	}

	// typing the filter narrows the friends as the query changes
	if s.content.filter.Typing() {
		changed, cmd := s.content.filter.Update(msg)
		if changed {
			s.content.friends.Reset(s.friendItems()...)
		}

		return s, cmd
	}

	// moving to another friend drops pending logout and unlink confirmation, like any other key
	if ui.Moves(msg) {
		s.confirmLogout, s.confirmUnlink = false, false
	}

	_, cmd := s.content.friends.Update(msg)
	return s, cmd
}

// reloadFriends loads friends again, since they change on other screens. Like a poll, it logs a failure and keeps the
// shown friends.
func (s Screen) reloadFriends() tea.Cmd {
	if s.self == nil {
		return nil
	}

	current := s.user
	return func() tea.Msg {
		friends, err := s.service.friends(current)
		if err != nil {
			slog.Error("failed to reload friends", "err", err)
			return nil
		}

		return router.TargetMsg{Type: screen.TypeProfile, Inner: friendsMsg{user: current, friends: friends}}
	}
}

// friendItems builds a button for each friend that matches the filter. The button opens that friend's profile.
func (s Screen) friendItems() []ui.Component {
	listed := s.listed()
	items := make([]ui.Component, len(listed))
	for i, index := range listed {
		items[i] = ui.NewButton(s.friends[index].Nickname.Value(), screen.Send(openFriendMsg{index: index}))
	}

	return items
}

// listed returns indexes of friends whose nickname matches the filter. The list shows only nicknames, so only
// nicknames match.
func (s Screen) listed() []int {
	var listed []int
	for i, friend := range s.friends {
		if s.content.filter.Match(friend.Nickname.Value()) {
			listed = append(listed, i)
		}
	}

	return listed
}

// ask shows the field for p, empty and focused.
func (s *Screen) ask(p prompt) tea.Cmd {
	input := s.content.prompt.Raw()
	switch p {
	case promptInvite:
		input.Prompt, input.Placeholder, input.CharLimit = "invite link: ", "link or token", 0
	case promptEmail:
		input.Prompt, input.Placeholder, input.CharLimit = "email: ", "", 2048
	case promptCode:
		input.Prompt, input.Placeholder, input.CharLimit = "code: ", "8 digits from the email", 8
	}

	s.prompt = p
	input.SetValue("")
	_, cmd := s.content.prompt.Update(ui.FocusMsg{})
	return cmd
}

// stopPrompt hides the field and unfocuses it.
func (s *Screen) stopPrompt() {
	s.prompt, s.sending = promptNone, false
	s.content.prompt.Update(ui.UnfocusMsg{})
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

	invite := ""
	if s.invite.link != "" {
		invite = s.invite.link + ui.MutedStyle.Render(" expires "+s.invite.expiration.Local().Format("Mon 15:04"))
	}

	// long descriptions wrap to the window instead of running off its edge
	return lipgloss.NewStyle().Width(max(s.width, 20)).Render(ui.BoldStyle.Render(s.self.Nickname.Value()) + "\n" + ui.Fields(
		"email", email,
		"description", ui.Emojize(s.self.Description.Value()),
		"interests", strings.Join(interests, ", "),
		"social link", s.self.SocialLink.Value(),
		"invite link", invite))
}

// Unsaved reports whether the profile is being edited.
func (s Screen) Unsaved() bool {
	return s.editing
}

// Typing reports whether the edit form, the field under the profile or the filter takes typed text.
func (s Screen) Typing() bool {
	return s.editing || s.prompt != promptNone || s.content.filter.Typing()
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
	profile := s.profile()
	if profile != "" {
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

	if s.confirmUnlink {
		parts = append(parts, ui.DangerStyle.Render("Unlink your email? Without it, logging out loses this account for good. Press "+
			keys.Profile.UnlinkEmail.Key()+" again to confirm."))
	}

	if s.prompt != promptNone {
		s.content.prompt.Raw().SetWidth(max(s.width-10-lipgloss.Width(s.content.prompt.Raw().Prompt), 10))
		parts = append(parts, s.content.prompt.View())
	}

	if profile != "" {
		parts = append(parts, ui.BoldStyle.Render(fmt.Sprintf("friends (%d)", len(s.friends))))
		if len(s.friends) == 0 {
			parts = append(parts, ui.MutedStyle.Render("no friends yet"))
		}

		if filter := s.content.filter.View(s.width); filter != "" {
			parts = append(parts, filter)
		}

		if s.content.filter.Query() != "" && len(s.listed()) == 0 {
			parts = append(parts, ui.MutedStyle.Render("nobody matches"))
		}
	}

	header := strings.Join(parts, "\n\n")
	if s.content.friends.Len() == 0 {
		return header
	}

	// the friends list fills the height left under everything else, so the logout warning stays on screen
	s.content.friends.SetWidth(s.width)
	s.content.friends.SetHeight(max(s.height-lipgloss.Height(header)-1, 3))
	// the list starts below the header and the blank line after it
	s.content.friends.SetTop(lipgloss.Height(header) + 1)
	return lipgloss.JoinVertical(lipgloss.Left, header, "", s.content.friends.View())
}
