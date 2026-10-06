package profile

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/ui"
)

// Screen is a model of profile screen.
type Screen struct {
	service *Service

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

func (Screen) actions() []ui.Action {
	return []ui.Action{{Key: ui.Key("esc", "back"), Msg: screen.ChangeMsg{NewType: screen.TypeHome}}}
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
	case auth.LoginMsg:
		s.content.label.Set(ui.MutedStyle.Render("loading..."))
		return s, func() tea.Msg {
			self, err := s.service.get(msg.User)
			if err != nil {
				s.content.label.Set(ui.DangerStyle.Render(fmt.Sprintf("error loading profile: %s", err.Error())))
				return screen.TickMsg{}
			}

			var interests strings.Builder
			interestsSlice := self.Interests.Value()
			for i, interest := range interestsSlice {
				interests.WriteString(interest.Value())
				if i != len(interestsSlice)-1 {
					interests.WriteString(", ")
				}
			}

			s.content.label.Set(ui.BoldStyle.Render(self.Nickname.Value()) + "\n" + ui.Fields(
				"description", self.Description.Value(),
				"interests", interests.String(),
				"social link", self.SocialLink.Value()))

			return screen.TickMsg{}
		}
	}

	return s, nil
}

func (s Screen) View() string {
	return s.content.label.View()
}
