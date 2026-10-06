package people

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
)

// refreshMsg asks people screen to reload people for the current user.
type refreshMsg struct{}

// loadedMsg carries loaded people, always wrapped into router.TargetMsg so it reaches this screen even after leaving it.
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
		label *ui.Label
		list  *ui.List
	}

	width  int
	height int
}

// New creates new Screen from Service.
func New(service *Service) Screen {
	result := Screen{
		service: service,
	}

	result.content.label = ui.NewLabel(ui.MutedStyle.Render("log in to see people"))

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
		s.content.label.Set(ui.MutedStyle.Render("log in to see people"))
		return nil
	}

	s.content.label.Set(ui.MutedStyle.Render("loading..."))
	return func() tea.Msg {
		entries, err := s.service.get(s.user)
		return router.TargetMsg{Type: screen.TypePeople, Inner: loadedMsg{entries: entries, err: err}}
	}
}

// items builds list of people.
func (s Screen) items() []tea.Model {
	items := make([]tea.Model, len(s.entries))
	for i, entry := range s.entries {
		items[i] = ui.NewLabel(s.card(entry))
	}

	return items
}

func (Screen) actions() []ui.Action {
	return []ui.Action{
		{Key: ui.Key("r", "refresh"), Msg: refreshMsg{}},
		{Key: ui.Key("esc", "back"), Msg: screen.ChangeMsg{NewType: screen.TypeHome}},
	}
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
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
		switch {
		case msg.err != nil:
			s.content.label.Set(ui.DangerStyle.Render(fmt.Sprintf("error loading people: %s", msg.err.Error())))
			return s, nil
		case len(msg.entries) == 0:
			s.content.label.Set(ui.MutedStyle.Render("you are all caught up"))
		default:
			s.content.label.Set("")
		}

		s.entries = msg.entries
		s.content.list.Reset(s.items()...)
		return s, nil
	case auth.LoginMsg:
		s.user = msg.User
		return s, s.load()
	case refreshMsg:
		return s, s.load()
	}

	_, cmd := s.content.list.Update(msg)
	return s, cmd
}

func (s Screen) View() string {
	status := s.content.label.View()
	if status == "" {
		s.content.list.SetHeight(s.height)
		return s.content.list.View()
	}

	s.content.list.SetHeight(max(s.height-lipgloss.Height(status)-1, 3))
	return lipgloss.JoinVertical(lipgloss.Left, status, "", s.content.list.View())
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

	// leave room for list marker plus slack for emoji that terminals draw wider than measured,
	// zero width means no wrapping before the first WindowSizeMsg
	width := 0
	if s.width > 0 {
		width = max(s.width-6, 20)
	}

	body := lipgloss.NewStyle().PaddingLeft(2).Width(width).Render(strings.Join(lines, "\n"))
	return ui.BoldStyle.Render(details.Nickname.Value()) + "\n" + body
}
