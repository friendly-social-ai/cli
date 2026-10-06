package community

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
)

type mode int

const (
	modeList mode = iota
	modePost
)

// Messages produced by buttons of the screen.
type (
	submitMsg     struct{}
	refreshMsg    struct{}
	moreMsg       struct{}
	backMsg       struct{}
	editMsg       struct{}
	cancelEditMsg struct{}
	deleteMsg     struct{}
	openMsg       struct{ post sdk.CommunityPostDescriptor }
)

// Messages produced by requests, always wrapped into router.TargetMsg so they reach this screen even after leaving it.
type (
	listMsg struct {
		page   *sdk.Cursor[sdk.CommunityPost]
		append bool
	}
	detailsMsg struct{ details *sdk.CommunityPostDetails }
	repliesMsg struct {
		page *sdk.Cursor[sdk.CommunityPostReply]
	}
	doneMsg   struct{}
	failedMsg struct{ err error }
)

// Screen is a model of community screen: list of posts and details of a single post.
type Screen struct {
	service *Service
	user    *sdk.Authorization
	mode    mode

	posts []sdk.CommunityPost
	next  *sdk.CursorId

	details     *sdk.CommunityPostDetails
	replies     []sdk.CommunityPostReply
	repliesNext *sdk.CursorId

	editing       bool
	confirmDelete bool

	content struct {
		status *ui.Label
		field  *ui.Field
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

	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 4096

	result.content.status = ui.NewLabel("log in to see community")
	result.content.field = ui.NewField(input)
	result.content.list = ui.NewList()
	result.content.list.Reset(result.items()...)

	return result
}

func (Screen) ID() screen.Type {
	return screen.TypeCommunity
}

func (s Screen) Init() tea.Cmd {
	return nil
}

func send(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

func (s Screen) request(status string, fn func() (tea.Msg, error)) tea.Cmd {
	if s.user == nil {
		s.content.status.Set("log in to see community")
		return nil
	}

	s.content.status.Set(status)
	return func() tea.Msg {
		msg, err := fn()
		if err != nil {
			msg = failedMsg{err: err}
		}

		return router.TargetMsg{Type: screen.TypeCommunity, Inner: msg}
	}
}

func (s Screen) loadList(cursor *sdk.CursorId) tea.Cmd {
	return s.request("loading posts...", func() (tea.Msg, error) {
		page, err := s.service.list(s.user, cursor)
		return listMsg{page: page, append: cursor != nil}, err
	})
}

func (s Screen) loadDetails(post sdk.CommunityPostDescriptor) tea.Cmd {
	return s.request("loading post...", func() (tea.Msg, error) {
		details, err := s.service.details(s.user, post)
		return detailsMsg{details: details}, err
	})
}

func (s Screen) loadReplies() tea.Cmd {
	post, cursor := s.details.Post.Descriptor(), s.repliesNext
	return s.request("loading replies...", func() (tea.Msg, error) {
		page, err := s.service.replies(s.user, post, cursor)
		return repliesMsg{page: page}, err
	})
}

func (s Screen) submit() tea.Cmd {
	text := s.content.field.Value()

	if s.mode == modeList {
		return s.request("posting...", func() (tea.Msg, error) {
			return doneMsg{}, s.service.post(s.user, text, nil)
		})
	}

	post := s.details.Post
	if s.editing {
		return s.request("saving...", func() (tea.Msg, error) {
			return doneMsg{}, s.service.edit(s.user, post.Id, text)
		})
	}

	replyTo := post.Descriptor()
	return s.request("replying...", func() (tea.Msg, error) {
		return doneMsg{}, s.service.post(s.user, text, &replyTo)
	})
}

func (s Screen) delete() tea.Cmd {
	id := s.details.Post.Id
	return s.request("deleting...", func() (tea.Msg, error) {
		return doneMsg{}, s.service.delete(s.user, id)
	})
}

func (s Screen) reload() tea.Cmd {
	if s.mode == modePost {
		return s.loadDetails(s.details.Post.Descriptor())
	}

	return s.loadList(nil)
}

func (s Screen) owns(post sdk.CommunityPost) bool {
	return s.user != nil && post.Owner != nil && post.Owner.Id == s.user.Id
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.content.list.Set(s.items()...)
		return s, nil
	case auth.LoginMsg:
		s.user = msg.User
		s.mode = modeList
		return s, s.loadList(nil)
	case refreshMsg:
		return s, s.reload()
	case moreMsg:
		if s.mode == modePost {
			return s, s.loadReplies()
		}

		return s, s.loadList(s.next)
	case openMsg:
		return s, s.loadDetails(msg.post)
	case backMsg:
		if s.mode == modeList {
			return s, send(screen.ChangeMsg{NewType: screen.TypeHome})
		}

		s.mode = modeList
		s.details = nil
		s.editing = false
		s.content.field.Raw().SetValue("")
		s.content.status.Set("")
		s.content.list.Reset(s.items()...)
		return s, nil
	case editMsg:
		s.editing = true
		s.content.field.Raw().SetValue(s.details.Post.Text.Value())
		s.content.list.Reset(s.items()...)
		return s, nil
	case cancelEditMsg:
		s.editing = false
		s.content.field.Raw().SetValue("")
		s.content.list.Reset(s.items()...)
		return s, nil
	case deleteMsg:
		if s.confirmDelete {
			return s, s.delete()
		}

		s.confirmDelete = true
		s.content.list.Set(s.items()...)
		return s, nil
	case submitMsg:
		return s, s.submit()
	case listMsg:
		s.next = msg.page.NextId
		s.content.status.Set("")
		if msg.append {
			s.posts = append(s.posts, msg.page.Data...)
			s.content.list.Set(s.items()...)
			return s, nil
		}

		s.posts = msg.page.Data
		s.content.list.Reset(s.items()...)
		return s, nil
	case detailsMsg:
		s.mode = modePost
		s.details = msg.details
		s.replies = msg.details.Replies.Data
		s.repliesNext = msg.details.Replies.NextId
		s.editing = false
		s.confirmDelete = false
		s.content.field.Raw().SetValue("")
		s.content.status.Set("")
		s.content.list.Reset(s.items()...)
		return s, nil
	case repliesMsg:
		s.replies = append(s.replies, msg.page.Data...)
		s.repliesNext = msg.page.NextId
		s.content.status.Set("")
		s.content.list.Set(s.items()...)
		return s, nil
	case doneMsg:
		s.content.field.Raw().SetValue("")
		return s, s.reload()
	case failedMsg:
		s.content.status.Set("error: " + msg.err.Error())
		return s, nil
	}

	_, cmd := s.content.list.Update(msg)
	return s, cmd
}
