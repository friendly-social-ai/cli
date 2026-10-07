package activity

import (
	"log"
	"strconv"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/screen/community"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
)

// Messages of the screen.
type (
	refreshMsg struct{}
	moreMsg    struct{}
	openMsg    struct{ index int }
	readAllMsg struct{}
	// loadedMsg carries a page of activity. Requests wrap it into router.TargetMsg so it reaches this screen anywhere.
	loadedMsg struct {
		page   *sdk.Cursor[sdk.Activity]
		append bool
		err    error
	}
	// polledMsg carries the first page of activity checked in the background
	polledMsg struct{ page *sdk.Cursor[sdk.Activity] }
)

// Screen is a model of activity screen, which lists replies to user's posts.
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

	result.content.status = ui.NewLabel("")
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
		actions = append(actions, ui.Action{Key: ui.Key("l", "open", "enter")})
	}

	if s.unreadCount() > 0 {
		actions = append(actions, ui.Action{Key: ui.Key("m", "mark all read"), Msg: readAllMsg{}})
	}

	return append(actions, ui.Action{Key: ui.Key("r", "refresh"), Msg: refreshMsg{}})
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
}

func (s Screen) Status() string {
	return s.content.status.View()
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

// poll fetches the first page of activity without showing a status. On failure it logs and tries again next minute.
func (s Screen) poll() tea.Cmd {
	if s.user == nil {
		return nil
	}

	user := s.user
	return func() tea.Msg {
		page, err := s.service.list(user, nil)
		if err != nil {
			log.Printf("error: %v", err)
			return nil
		}

		return router.TargetMsg{Type: screen.TypeActivity, Inner: polledMsg{page: page}}
	}
}

// unreadCount returns number of unread activities.
func (s Screen) unreadCount() int {
	count := 0
	for _, activity := range s.activities {
		if !activity.IsRead {
			count++
		}
	}

	return count
}

// Badge returns number of unread activities for the tab, empty when all are read.
func (s Screen) Badge() string {
	if count := s.unreadCount(); count > 0 {
		return strconv.Itoa(count)
	}

	return ""
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
		s.content.status.Set("")
		s.content.list.Reset(s.items()...)
		return s, nil
	case refreshMsg:
		return s, s.load(nil)
	case screen.MinuteMsg:
		s.content.list.Set(s.items()...)
		return s, s.poll()
	case polledMsg:
		// new activities go on top and the cursor stays on the same one
		known := make(map[sdk.ActivityId]bool, len(s.activities))
		for _, activity := range s.activities {
			known[activity.Id] = true
		}

		var fresh []sdk.Activity
		for _, activity := range msg.page.Data {
			if !known[activity.Id] {
				fresh = append(fresh, activity)
			}
		}

		if len(fresh) == 0 {
			return s, nil
		}

		cursor, offset := s.content.list.Position()
		s.activities = append(fresh, s.activities...)
		s.content.list.Set(s.items()...)
		s.content.list.SetPosition(cursor+len(fresh), offset)
		return s, nil
	case moreMsg:
		if s.loadingMore || s.next == nil {
			return s, nil
		}

		s.loadingMore = true
		return s, s.load(s.next)
	case loadedMsg:
		s.loadingMore = false
		if msg.err != nil {
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
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
		return s, nil
	case readAllMsg:
		var ids []sdk.ActivityId
		for i := range s.activities {
			if !s.activities[i].IsRead {
				s.activities[i].IsRead = true
				ids = append(ids, s.activities[i].Id)
			}
		}

		s.content.list.Set(s.items()...)
		user := s.user
		// failed reads are ignored, as when opening one
		return s, func() tea.Msg {
			for _, id := range ids {
				_ = s.service.read(user, id)
			}

			return nil
		}
	case openMsg:
		activity := &s.activities[msg.index]
		cmds := []tea.Cmd{
			screen.Send(router.TargetMsg{Type: screen.TypeCommunity, Inner: community.OpenMsg{
				Post: activity.Post.Descriptor(),
				From: screen.TypeActivity,
			}}),
			screen.Send(screen.ChangeMsg{NewType: screen.TypeCommunity}),
		}

		// like the web, opening marks it read right away, and a failed read is ignored like a missed notification
		if !activity.IsRead {
			activity.IsRead = true
			s.content.list.Set(s.items()...)
			user, id := s.user, activity.Id
			cmds = append(cmds, func() tea.Msg {
				_ = s.service.read(user, id)
				return nil
			})
		}

		return s, tea.Batch(cmds...)
	}

	_, cmd := s.content.list.Update(msg)

	// load the next page when the cursor gets close to the end, or the wheel scrolls the end on screen
	if ui.Moves(msg) && (s.content.list.Cursor() >= s.content.list.Len()-3 || s.content.list.AtEnd()) {
		model, more := s.Update(moreMsg{})
		return model, tea.Batch(cmd, more)
	}

	return s, cmd
}

// items builds list of activities, unread ones marked with a dot.
func (s Screen) items() []ui.Component {
	width := 74
	if s.width > 0 {
		width = max(s.width-6, 20)
	}

	items := make([]ui.Component, len(s.activities))
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
	switch {
	case s.user == nil:
		return ui.MutedStyle.Render("log in to see activity")
	case len(s.activities) == 0 && s.Status() == "":
		return ui.MutedStyle.Render("nothing here yet")
	}

	s.content.list.SetHeight(s.height)
	return s.content.list.View()
}
