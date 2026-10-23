package activity

import (
	"log/slog"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social-ai/cli/internal/keys"
	"github.com/friendly-social-ai/cli/internal/router"
	"github.com/friendly-social-ai/cli/internal/screen"
	"github.com/friendly-social-ai/cli/internal/screen/auth"
	"github.com/friendly-social-ai/cli/internal/screen/community"
	"github.com/friendly-social-ai/cli/internal/ui"
	sdk "github.com/friendly-social-ai/golang-sdk"
)

// Messages of the screen.
type (
	refreshMsg struct{}
	moreMsg    struct{}
	openMsg    struct{ index int }
	readAllMsg struct{}
	// nextUnreadMsg selects the next unread activity after the cursor. Past the last one it starts again from the top.
	nextUnreadMsg struct{}
	// loadedMsg carries a page of activity. For the first page it also tells whether an unseen activity is waiting.
	// Requests wrap it into router.TargetMsg so it reaches this screen anywhere.
	loadedMsg struct {
		page   *sdk.Cursor[sdk.Activity]
		append bool
		unseen bool
		err    error
	}
	// polledMsg carries the first page of activity checked in the background, and whether an unseen activity is waiting
	polledMsg struct {
		page   *sdk.Cursor[sdk.Activity]
		unseen bool
	}
)

// Screen is a model of activity screen, which lists replies to user's posts and to threads user subscribes to.
type Screen struct {
	service *Service
	user    *sdk.Authorization

	activities  []sdk.Activity
	next        *sdk.CursorId
	loadingMore bool

	// shown tells that the screen is the current one. unseen tells that an unread activity arrived since the last
	// acknowledge. The tab shows it as a dot.
	shown  bool
	unseen bool

	content struct {
		status *ui.Status
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
		actions = append(actions, ui.Action{Key: keys.Hint("scroll", keys.Label(keys.Navigation.HalfPageDown, keys.Navigation.HalfPageUp, "/"))})
	}

	if len(s.activities) > 0 {
		actions = append(actions, ui.Action{Key: keys.Bind("open", keys.Navigation.Open)})
	}

	if s.unreadCount() > 0 {
		actions = append(actions,
			ui.Action{Key: keys.Bind("next unread", keys.Activity.NextUnread), Msg: nextUnreadMsg{}},
			ui.Action{Key: keys.Bind("mark all read", keys.Activity.ReadAll), Msg: readAllMsg{}})
	}

	return append(actions, ui.Action{Key: keys.Bind("refresh", keys.Common.Refresh), Msg: refreshMsg{}})
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
}

// Position returns the selected item, counting from 1, and the number of items. The footer shows both.
func (s Screen) Position() (int, int) {
	return s.content.list.Cursor() + 1, s.content.list.Len()
}

func (s Screen) Status() *ui.Status {
	return s.content.status
}

func (s Screen) load(cursor *sdk.CursorId) tea.Cmd {
	if s.user == nil {
		return nil
	}

	if cursor == nil {
		s.content.status.Busy("loading")
	}

	user := s.user
	return func() tea.Msg {
		page, err := s.service.list(user, cursor)
		unseen := err == nil && cursor == nil && s.checkUnseen(user)
		return router.TargetMsg{Type: screen.TypeActivity, Inner: loadedMsg{page: page, append: cursor != nil, unseen: unseen, err: err}}
	}
}

// poll fetches the first page of activity without showing a status. On failure it logs and tries again at the next refresh.
func (s Screen) poll() tea.Cmd {
	if s.user == nil {
		return nil
	}

	user := s.user
	return func() tea.Msg {
		page, err := s.service.list(user, nil)
		if err != nil {
			slog.Error("failed to poll activity", "err", err)
			return nil
		}

		return router.TargetMsg{Type: screen.TypeActivity, Inner: polledMsg{page: page, unseen: s.checkUnseen(user)}}
	}
}

// checkUnseen reports whether user has an unseen activity. On failure it logs and reports none, and the next refresh
// checks again.
func (s Screen) checkUnseen(user *sdk.Authorization) bool {
	unseen, err := s.service.unseen(user)
	if err != nil {
		slog.Error("failed to check unseen activity", "err", err)
	}

	return unseen
}

// notice shows the dot for an unseen activity, or acknowledges it right away while the screen is the current one.
func (s *Screen) notice(unseen bool) tea.Cmd {
	s.unseen = unseen && !s.shown
	if unseen && s.shown {
		return s.acknowledge()
	}

	return nil
}

// acknowledge marks the unseen activity as seen. Like a failed read, a failed acknowledge is ignored, and the dot
// comes back at the next refresh.
func (s Screen) acknowledge() tea.Cmd {
	user := s.user
	return func() tea.Msg {
		_ = s.service.acknowledge(user)
		return nil
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

// Badge returns a dot for the tab when an unseen activity is waiting, like the web.
func (s Screen) Badge() string {
	if s.unseen {
		return "●"
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
		s.user, s.activities, s.next, s.unseen = nil, nil, nil, false
		s.content.status.Set("")
		s.content.list.Reset(s.items()...)
		return s, nil
	case refreshMsg:
		return s, s.load(nil)
	case screen.MinuteMsg:
		s.content.list.Set(s.items()...)
		return s, nil
	case screen.PollMsg:
		return s, s.poll()
	case screen.ShownMsg:
		s.shown = true
		return s, s.notice(s.unseen)
	case screen.HiddenMsg:
		s.shown = false
		return s, nil
	case polledMsg:
		cmd := s.notice(msg.unseen)

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
			return s, cmd
		}

		cursor, offset := s.content.list.Position()
		s.activities = append(fresh, s.activities...)
		s.content.list.Set(s.items()...)
		s.content.list.SetPosition(cursor+len(fresh), offset)
		return s, cmd
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
		var cmd tea.Cmd
		if msg.append {
			s.activities = append(s.activities, msg.page.Data...)
			s.content.list.Set(s.items()...)
		} else {
			s.activities = msg.page.Data
			s.content.list.Reset(s.items()...)
			cmd = s.notice(msg.unseen)
		}

		s.content.status.Set("")
		return s, cmd
	case nextUnreadMsg:
		cursor := s.content.list.Cursor()
		for i := 1; i <= len(s.activities); i++ {
			if j := (cursor + i) % len(s.activities); !s.activities[j].IsRead {
				s.content.list.Select(j)
				break
			}
		}

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
		// new replies under a post open at their common denominator, like the web
		post := *activity.Post
		if activity.CommonDenominator != nil {
			post = *activity.CommonDenominator
		}

		// the post opens before the screen changes, so the previous view of community never shows
		cmds := []tea.Cmd{tea.Sequence(
			screen.Send(router.TargetMsg{Type: screen.TypeCommunity, Inner: community.OpenMsg{
				Post: post,
				From: screen.TypeActivity,
			}}),
			screen.Send(screen.ChangeMsg{NewType: screen.TypeCommunity}),
		)}

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

// items builds list of activities, unread ones marked with a trailing dot.
func (s Screen) items() []ui.Component {
	width := 74
	if s.width > 0 {
		width = max(s.width-6, 20)
	}

	items := make([]ui.Component, len(s.activities))
	for i, activity := range s.activities {
		var prefix string
		switch activity.Type {
		case "new_reply":
			prefix = "Reply from "
		case "indirect_replies":
			prefix = "New replies under post from "
		}

		if prefix == "" || activity.Post == nil || activity.Post.Owner == nil {
			items[i] = ui.NewLabel(ui.MutedStyle.Render("unsupported activity"))
			continue
		}

		title := prefix + ui.BoldStyle.Render(activity.Post.Owner.Nickname.Value()) +
			ui.MutedStyle.Render(" · "+community.Ago(activity.Instant))
		if activity.IsRead {
			title = ansi.Truncate(title, width, "…")
		} else {
			title = ansi.Truncate(title, width-2, "…") + ui.AccentStyle.Render(" ●")
		}

		line := ansi.Truncate(community.FirstLine(*activity.Post), width, "…")
		items[i] = ui.NewButton(title+"\n"+line, screen.Send(openMsg{index: i}))
	}

	return items
}

func (s Screen) View() string {
	switch {
	case s.user == nil:
		return ui.MutedStyle.Render("log in to see activity")
	case len(s.activities) == 0 && s.content.status.Value() == "":
		return ui.Empty(s.width, s.height, "no activity yet", keys.Common.Refresh.Key(), "to refresh")
	}

	s.content.list.SetHeight(s.height)
	s.content.list.SetWidth(s.width)
	return s.content.list.View()
}
