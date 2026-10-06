package people

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
)

// refreshMsg asks people screen to reload people for the current user.
type refreshMsg struct{}

// Messages produced by key actions on the selected person.
type (
	connectMsg struct{}
	skipMsg    struct{}
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
		status *ui.Label
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

	result.content.status = ui.NewLabel("")

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

	s.content.status.Set(ui.MutedStyle.Render("loading..."))
	return func() tea.Msg {
		entries, err := s.service.get(s.user)
		return router.TargetMsg{Type: screen.TypePeople, Inner: loadedMsg{entries: entries, err: err}}
	}
}

// items builds list of people.
func (s Screen) items() []ui.Component {
	items := make([]ui.Component, len(s.entries))
	for i, entry := range s.entries {
		items[i] = ui.NewLabel(s.card(entry))
	}

	return items
}

func (s Screen) actions() []ui.Action {
	var actions []ui.Action
	if s.content.list.Scrollable() {
		actions = append(actions, ui.Action{Key: ui.Key("ctrl+d/u", "scroll")})
	}

	if len(s.entries) > 0 {
		desc := "connect"
		if s.entries[s.content.list.Cursor()].IsRequest {
			desc = "accept"
		}

		actions = append(actions,
			ui.Action{Key: ui.Key("a", desc), Msg: connectMsg{}},
			ui.Action{Key: ui.Key("x", "skip"), Msg: skipMsg{}})
	}

	return append(actions, ui.Action{Key: ui.Key("r", "refresh"), Msg: refreshMsg{}})
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
}

func (s Screen) Status() string {
	return s.content.status.View()
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
		s.content.list.Reset(s.items()...)
		return s, nil
	case auth.LoginMsg:
		s.user = msg.User
		return s, s.load()
	case refreshMsg:
		return s, s.load()
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

	_, cmd := s.content.list.Update(msg)
	return s, cmd
}

// take removes the selected person right away and sends request for them. A failed request brings them back.
func (s Screen) take(request func(*sdk.Authorization, sdk.UserDetails) error) (screen.Model, tea.Cmd) {
	index := s.content.list.Cursor()
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
	case len(s.entries) == 0 && s.Status() == "":
		return ui.MutedStyle.Render("you are all caught up")
	}

	s.content.list.SetHeight(s.height)
	return s.content.list.View()
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

	lines := []string{details.Description.Value()}
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

	body := lipgloss.NewStyle().PaddingLeft(2).Width(width).Render(strings.Join(lines, "\n"))
	return ui.BoldStyle.Render(details.Nickname.Value()) + "\n" + body
}
