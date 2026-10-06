package activity

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/screen/community"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
)

// UnreadMsg tells how many activities are unread, broadcast whenever it changes.
type UnreadMsg struct {
	Count int
}

// Messages of the screen.
type (
	refreshMsg struct{}
	moreMsg    struct{}
	openMsg    struct{ index int }
	// loadedMsg carries a page of activity, wrapped into router.TargetMsg so it reaches this screen anywhere.
	loadedMsg struct {
		page   *sdk.Cursor[sdk.Activity]
		append bool
		err    error
	}
)

// Screen is a model of activity screen: replies to user's posts.
type Screen struct {
	service *Service
	user    *sdk.Authorization

	activities  []sdk.Activity
	next        *sdk.CursorId
	loadingMore bool

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

	result.content.status = ui.NewLabel(ui.MutedStyle.Render("log in to see activity"))
	result.content.list = ui.NewList()
	result.content.list.SetGap(1)

	return result
}

func (Screen) ID() screen.Type {
	return screen.TypeActivity
}

func (Screen) Init() tea.Cmd {
	return nil
}

func (s Screen) actions() []ui.Action {
	var actions []ui.Action
	if s.content.list.Scrollable() {
		actions = append(actions, ui.Action{Key: ui.Key("ctrl+d/u", "scroll")})
	}

	if len(s.activities) > 0 {
		actions = append(actions, ui.Action{Key: ui.Key("enter", "open")})
	}

	return append(actions,
		ui.Action{Key: ui.Key("r", "refresh"), Msg: refreshMsg{}},
		ui.Action{Key: ui.Key("esc", "back"), Msg: screen.ChangeMsg{NewType: screen.TypeHome}})
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
}

func (s Screen) load(cursor *sdk.CursorId) tea.Cmd {
	if s.user == nil {
		return nil
	}

	if cursor == nil {
		s.content.status.Set(ui.MutedStyle.Render("loading..."))
	}

	user := s.user
	return func() tea.Msg {
		page, err := s.service.list(user, cursor)
		return router.TargetMsg{Type: screen.TypeActivity, Inner: loadedMsg{page: page, append: cursor != nil, err: err}}
	}
}

// unread broadcasts number of unread activities.
func (s Screen) unread() tea.Cmd {
	count := 0
	for _, activity := range s.activities {
		if !activity.IsRead {
			count++
		}
	}

	return screen.Send(router.BroadcastMsg{Inner: UnreadMsg{Count: count}})
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
	case auth.LoginMsg:
		s.user = msg.User
		return s, s.load(nil)
	case auth.LogoutMsg:
		s.user, s.activities, s.next = nil, nil, nil
		s.content.status.Set(ui.MutedStyle.Render("log in to see activity"))
		s.content.list.Reset(s.items()...)
		return s, nil
	case refreshMsg:
		return s, s.load(nil)
	case moreMsg:
		if s.loadingMore || s.next == nil {
			return s, nil
		}

		s.loadingMore = true
		return s, s.load(s.next)
	case loadedMsg:
		s.loadingMore = false
		if msg.err != nil {
			s.content.status.Set(ui.DangerStyle.Render("error: " + msg.err.Error()))
			return s, nil
		}

		s.next = msg.page.NextId
		if msg.append {
			s.activities = append(s.activities, msg.page.Data...)
			s.content.list.Set(s.items()...)
		} else {
			s.activities = msg.page.Data
			s.content.list.Reset(s.items()...)
		}

		s.content.status.Set("")
		if len(s.activities) == 0 {
			s.content.status.Set(ui.MutedStyle.Render("nothing here yet"))
		}

		return s, s.unread()
	case openMsg:
		activity := &s.activities[msg.index]
		cmds := []tea.Cmd{
			screen.Send(router.TargetMsg{Type: screen.TypeCommunity, Inner: community.OpenMsg{
				Post: activity.Post.Descriptor(),
				From: screen.TypeActivity,
			}}),
			screen.Send(screen.ChangeMsg{NewType: screen.TypeCommunity}),
		}

		// same as web: opening marks it read right away, failing quietly like a missed notification
		if !activity.IsRead {
			activity.IsRead = true
			s.content.list.Set(s.items()...)
			user, id := s.user, activity.Id
			cmds = append(cmds, s.unread(), func() tea.Msg {
				_ = s.service.read(user, id)
				return nil
			})
		}

		return s, tea.Batch(cmds...)
	}

	_, cmd := s.content.list.Update(msg)

	// load the next page when the cursor gets close to the end
	if _, ok := msg.(ui.MoveMsg); ok && s.content.list.Cursor() >= s.content.list.Len()-3 {
		model, more := s.Update(moreMsg{})
		return model, tea.Batch(cmd, more)
	}

	return s, cmd
}

// items builds list of activities, unread ones marked with a dot.
func (s Screen) items() []tea.Model {
	width := 74
	if s.width > 0 {
		width = max(s.width-6, 20)
	}

	items := make([]tea.Model, len(s.activities))
	for i, activity := range s.activities {
		if activity.Type != "reply" || activity.Post == nil || activity.Post.Owner == nil {
			items[i] = ui.NewLabel(ui.MutedStyle.Render("unsupported activity"))
			continue
		}

		dot := "  "
		if !activity.IsRead {
			dot = ui.AccentStyle.Render("● ")
		}

		title := dot + "Reply from " + ui.BoldStyle.Render(activity.Post.Owner.Nickname.Value()) +
			ui.MutedStyle.Render(" · "+community.Ago(activity.Instant))
		line := "  " + community.FirstLine(*activity.Post)
		items[i] = ui.NewButton(ansi.Truncate(title, width, "…")+"\n"+ansi.Truncate(line, width, "…"),
			screen.Send(openMsg{index: i}))
	}

	return items
}

func (s Screen) View() string {
	status := s.content.status.View()
	if status == "" {
		s.content.list.SetHeight(s.height)
		return s.content.list.View()
	}

	s.content.list.SetHeight(max(s.height-lipgloss.Height(status)-1, 3))
	return lipgloss.JoinVertical(lipgloss.Left, status, "", s.content.list.View())
}
