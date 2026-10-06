package people

import (
	"fmt"
	"strings"

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

// Screen is a model of people screen.
type Screen struct {
	service *Service
	user    *sdk.Authorization

	content struct {
		label *ui.Label
		list  *ui.List

		button struct {
			refresh *ui.Button
			home    *ui.Button
		}
	}
}

// New creates new Screen from Service.
func New(service *Service) Screen {
	result := Screen{
		service: service,
	}

	result.content.label = ui.NewLabel(ui.MutedStyle.Render("log in to see people"))
	result.content.button.refresh = ui.NewButton("Refresh", func() tea.Msg {
		return refreshMsg{}
	})
	result.content.button.home = ui.NewButton("Back", func() tea.Msg {
		return screen.ChangeMsg{NewType: screen.TypeHome}
	})

	result.content.list = ui.NewList(
		result.content.button.refresh,
		result.content.button.home)

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
		if err != nil {
			s.content.label.Set(ui.DangerStyle.Render(fmt.Sprintf("error loading people: %s", err.Error())))
			return screen.TickMsg{}
		}

		s.content.label.Set(render(entries))
		return screen.TickMsg{}
	}
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
	return lipgloss.JoinVertical(lipgloss.Left,
		s.content.label.View(),
		"",
		s.content.list.View())
}

func render(entries []sdk.FeedEntry) string {
	if len(entries) == 0 {
		return ui.MutedStyle.Render("you are all caught up")
	}

	views := make([]string, len(entries))
	for i, entry := range entries {
		details := entry.Details

		interests := make([]string, len(details.Interests.Value()))
		for j, interest := range details.Interests.Value() {
			interests[j] = interest.Value()
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

		body := lipgloss.NewStyle().PaddingLeft(2).Render(strings.Join(lines, "\n"))
		views[i] = ui.BoldStyle.Render(details.Nickname.Value()) + "\n" + body
	}

	return strings.Join(views, "\n\n")
}
