package people

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/friendly-social-ai/cli/internal/router"
	"github.com/friendly-social-ai/cli/internal/screen"
	"github.com/friendly-social-ai/cli/internal/screen/auth"
	"github.com/friendly-social-ai/cli/internal/ui"
	sdk "github.com/friendly-social-ai/golang-sdk"
)

// refreshMsg asks people screen to reload people for the current user.
type refreshMsg struct{}

// Messages produced by key actions on the selected person.
type (
	connectMsg     struct{}
	skipMsg        struct{}
	filterMsg      struct{}
	filterDoneMsg  struct{}
	clearFilterMsg struct{}
)

// failedMsg brings back the person at index whose request failed.
type failedMsg struct {
	index int
	entry sdk.FeedEntry
	err   error
}

// loadedMsg carries loaded people. The request wraps it into router.TargetMsg so it reaches this screen even after the user leaves.
type loadedMsg struct {
	entries []sdk.FeedEntry
	err     error
}

// Screen is a model of people screen.
type Screen struct {
	service *Service
	user    *sdk.Authorization
	entries []sdk.FeedEntry

	content struct {
		status *ui.Status
		filter *ui.Filter
		list   *ui.List
	}

	width  int
	height int
}

// New creates new Screen from Service.
func New(service *Service) Screen {
	result := Screen{
		service: service,
	}

	result.content.status = ui.NewStatus()
	result.content.filter = ui.NewFilter()

	result.content.list = ui.NewList()
	result.content.list.SetGap(1)
	result.content.list.Reset(result.items()...)

	return result
}

func (Screen) ID() screen.Type {
	return screen.TypePeople
}

func (s Screen) Init() tea.Cmd {
	return func() tea.Msg {
		return router.TargetMsg{Type: s.ID(), Inner: ui.SelectMsg{}}
	}
}

func (s Screen) load() tea.Cmd {
	if s.user == nil {
		return nil
	}

	s.content.status.Busy("loading")
	return func() tea.Msg {
		entries, err := s.service.get(s.user)
		return router.TargetMsg{Type: screen.TypePeople, Inner: loadedMsg{entries: entries, err: err}}
	}
}

// items builds list of people.
func (s Screen) items() []ui.Component {
	listed := s.listed()
	items := make([]ui.Component, len(listed))
	for i, index := range listed {
		items[i] = ui.NewLabel(s.card(s.entries[index]))
	}

	return items
}

// listed returns indexes of entries that match the filter by nickname, description or interests.
func (s Screen) listed() []int {
	var listed []int
	for i, entry := range s.entries {
		details := entry.Details
		text := []string{details.Nickname.Value(), details.Description.Value()}
		for _, interest := range details.Interests.Value() {
			text = append(text, interest.Value())
		}

		if s.content.filter.Match(strings.Join(text, " ")) {
			listed = append(listed, i)
		}
	}

	return listed
}

func (s Screen) actions() []ui.Action {
	if s.content.filter.Typing() {
		return []ui.Action{{Key: ui.Key("enter", "done", "esc"), Msg: filterDoneMsg{}}}
	}

	var actions []ui.Action
	if s.content.list.Scrollable() {
		actions = append(actions, ui.Action{Key: ui.Key("ctrl+d/u", "scroll")})
	}

	if listed := s.listed(); len(listed) > 0 {
		desc := "connect"
		if s.entries[listed[s.content.list.Cursor()]].IsRequest {
			desc = "accept"
		}

		actions = append(actions,
			ui.Action{Key: ui.Key("a", desc), Msg: connectMsg{}},
			ui.Action{Key: ui.Key("x", "skip"), Msg: skipMsg{}})
	}

	actions = append(actions,
		ui.Action{Key: ui.Key("/", "filter"), Msg: filterMsg{}},
		ui.Action{Key: ui.Key("r", "refresh"), Msg: refreshMsg{}})
	if s.content.filter.Query() != "" {
		actions = append(actions, ui.Action{Key: ui.Key("esc", "clear"), Msg: clearFilterMsg{}})
	}

	return actions
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
}

// Typing reports whether the filter takes typed text.
func (s Screen) Typing() bool {
	return s.content.filter.Typing()
}

func (s Screen) Status() *ui.Status {
	return s.content.status
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.ActionMsg:
		if action := ui.Dispatch(s.actions(), msg); action != nil {
			return s, screen.Send(action)
		}
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.content.list.Set(s.items()...)
		return s, nil
	case loadedMsg:
		if msg.err != nil {
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
			return s, nil
		}

		s.content.status.Set("")
		s.entries = msg.entries
		s.content.list.Reset(s.items()...)
		return s, nil
	case auth.LogoutMsg:
		s.user, s.entries = nil, nil
		s.content.status.Set("")
		s.content.filter.Clear()
		s.content.list.Reset(s.items()...)
		return s, nil
	case auth.LoginMsg:
		s.user = msg.User
		return s, s.load()
	case refreshMsg:
		return s, s.load()
	case filterMsg:
		s.content.filter.Start()
		return s, nil
	case filterDoneMsg:
		s.content.filter.Stop()
		return s, nil
	case clearFilterMsg:
		s.content.filter.Clear()
		s.content.list.Reset(s.items()...)
		return s, nil
	case connectMsg:
		return s.take(s.service.connect)
	case skipMsg:
		return s.take(s.service.skip)
	case failedMsg:
		s.entries = slices.Insert(s.entries, min(msg.index, len(s.entries)), msg.entry)
		s.content.list.Set(s.items()...)
		s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
		return s, nil
	}

	// typing the filter narrows the list as the query changes
	if s.content.filter.Typing() {
		changed, cmd := s.content.filter.Update(msg)
		if changed {
			s.content.list.Reset(s.items()...)
		}

		return s, cmd
	}

	_, cmd := s.content.list.Update(msg)
	return s, cmd
}

// take removes the selected person right away and sends request for them. A failed request brings them back.
func (s Screen) take(request func(*sdk.Authorization, sdk.UserDetails) error) (screen.Model, tea.Cmd) {
	index := s.listed()[s.content.list.Cursor()]
	entry := s.entries[index]
	s.entries = slices.Delete(s.entries, index, index+1)
	s.content.list.Set(s.items()...)
	s.content.status.Set("")

	user := s.user
	return s, func() tea.Msg {
		if err := request(user, entry.Details); err != nil {
			return router.TargetMsg{Type: screen.TypePeople, Inner: failedMsg{index: index, entry: entry, err: err}}
		}

		return nil
	}
}

func (s Screen) View() string {
	switch {
	case s.user == nil:
		return ui.MutedStyle.Render("log in to see people")
	case len(s.entries) == 0 && s.content.status.Value() == "":
		return ui.MutedStyle.Render("you are all caught up")
	}

	var top []string
	if filter := s.content.filter.View(s.width); filter != "" {
		top = append(top, filter)
	}

	if s.content.filter.Query() != "" && len(s.listed()) == 0 {
		top = append(top, ui.MutedStyle.Render("nobody matches"))
	}

	if len(top) == 0 {
		s.content.list.SetHeight(s.height)
		s.content.list.SetTop(0)
		return s.content.list.View()
	}

	header := strings.Join(top, "\n\n")
	s.content.list.SetHeight(max(s.height-lipgloss.Height(header)-1, 3))
	// the list starts below the header and the blank line after it
	s.content.list.SetTop(lipgloss.Height(header) + 1)
	return lipgloss.JoinVertical(lipgloss.Left, header, "", s.content.list.View())
}

// card renders a person wrapped to screen width.
func (s Screen) card(entry sdk.FeedEntry) string {
	details := entry.Details

	interests := make([]string, len(details.Interests.Value()))
	for i, interest := range details.Interests.Value() {
		interests[i] = interest.Value()
	}

	var tags []string
	if entry.IsRequest {
		tags = append(tags, "wants to be friends")
	}
	if entry.IsExtendedNetwork {
		tags = append(tags, "extended network")
	}
	if n := len(entry.CommonFriends); n > 0 {
		tags = append(tags, fmt.Sprintf("%d common friends", n))
	}

	lines := []string{ui.Emojize(details.Description.Value())}
	if fields := ui.Fields(
		"interests", strings.Join(interests, ", "),
		"social link", details.SocialLink.Value()); fields != "" {
		lines = append(lines, fields)
	}
	if len(tags) > 0 {
		lines = append(lines, ui.AccentStyle.Render(strings.Join(tags, " · ")))
	}

	// leave room for list marker plus slack for emoji that terminals draw wider than measured
	width := 0
	if s.width > 0 {
		width = max(s.width-6, 20)
	}

	body := lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
	return ui.BoldStyle.Render(details.Nickname.Value()) + "\n" + body
}
