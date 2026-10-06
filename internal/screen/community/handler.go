package community

import (
	"image"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/friendly-social/cli/internal/browser"
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

// OpenMsg asks community screen to open post. From is the screen to return to when leaving the post, empty for
// the community list.
type OpenMsg struct {
	Post sdk.CommunityPostDescriptor
	From screen.Type
}

// Messages produced by key actions of the screen.
type (
	attachMsg       struct{}
	pickMsg         struct{}
	cancelPickMsg   struct{}
	openLinkMsg     struct{ url string }
	openImageMsg    struct{ url string }
	attachDoneMsg   struct{}
	cancelDeleteMsg struct{}
	submitMsg       struct{}
	refreshMsg      struct{}
	moreMsg         struct{}
	backMsg         struct{}
	editMsg         struct{}
	cancelEditMsg   struct{}
	deleteMsg       struct{}
)

// Messages produced by requests. Requests wrap them into router.TargetMsg so they reach this screen even after the user leaves.
type (
	listMsg struct {
		page   *sdk.Cursor[sdk.CommunityPost]
		append bool
	}
	detailsMsg struct{ details *sdk.CommunityPostDetails }
	repliesMsg struct {
		page *sdk.Cursor[sdk.CommunityPostReply]
	}
	// doneMsg reports a finished write. posted is the new post, if any.
	doneMsg struct{ posted *sdk.CommunityPostDescriptor }
	// openedMsg reports that an image was opened in the image viewer.
	openedMsg struct{}
	// deletedMsg reports that the opened post was deleted.
	deletedMsg struct{}
	// attachedMsg carries URL of uploaded image for embedding into the post.
	attachedMsg struct{ url string }
	imageMsg    struct {
		url        string
		img        image.Image
		id         uint32
		cols, rows int
		upload     string
	}
	failedMsg struct{ err error }
)

// picture is an image of a post. img is nil when the download failed. A non-zero id means terminal graphics holds
// the upload and shows it as a cols x rows placeholder.
type picture struct {
	img        image.Image
	done       bool
	id         uint32
	cols, rows int
}

// postView is an opened post with its replies and list position, kept while the user opens one of its replies or
// its parent.
type postView struct {
	details        *sdk.CommunityPostDetails
	replies        []sdk.CommunityPostReply
	repliesNext    *sdk.CursorId
	cursor, offset int
}

// Screen is a model of community screen. It shows the list of posts and the details of a single post.
type Screen struct {
	service  *Service
	graphics *ui.Graphics
	markdown *ui.Markdown
	user     *sdk.Authorization
	mode     mode

	posts []sdk.CommunityPost
	next  *sdk.CursorId

	details     *sdk.CommunityPostDetails
	replies     []sdk.CommunityPostReply
	repliesNext *sdk.CursorId

	// pictures holds post images by URL, rendered caches their half-block drawings by URL and size.
	pictures map[string]*picture
	rendered map[string]string

	editing       bool
	confirmDelete bool
	loadingMore   bool
	attaching     bool
	picking       bool

	// from is the screen to return to when leaving the opened post
	from screen.Type

	// listCursor and listOffset keep the list position while a post is open
	listCursor, listOffset int
	// pending is the post to select once the next load arrives
	pending *sdk.CommunityPostId
	// stack keeps posts the user went through, so going back returns to each at its position
	stack []postView

	content struct {
		status *ui.Label
		field  *ui.TextArea
		prompt *ui.Field
		list   *ui.List
	}

	width  int
	height int
}

// New creates new Screen from Service. It draws images with graphics when it is not nil, and with half-blocks otherwise.
func New(service *Service, graphics *ui.Graphics) Screen {
	result := Screen{
		service:  service,
		graphics: graphics,
		markdown: ui.NewMarkdown(),
		pictures: make(map[string]*picture),
		rendered: make(map[string]string),
	}

	input := textarea.New()
	input.Prompt = ""
	input.ShowLineNumbers = false
	input.CharLimit = 4096
	input.SetHeight(3)

	result.content.status = ui.NewLabel("")
	result.content.field = ui.NewTextArea(input)

	prompt := textinput.New()
	prompt.Prompt = ""
	prompt.Placeholder = "Path to a png, jpeg or gif, or drop a file here"
	result.content.prompt = ui.NewField(prompt)
	result.content.list = ui.NewList()
	result.content.list.SetGap(1)
	result.content.list.Reset(result.items()...)

	return result
}

func (Screen) ID() screen.Type {
	return screen.TypeCommunity
}

func (s Screen) Init() tea.Cmd {
	return nil
}

func (s Screen) request(status string, fn func() (tea.Msg, error)) tea.Cmd {
	if s.user == nil {
		return nil
	}

	s.content.status.Set(ui.MutedStyle.Render(status))
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
			posted, err := s.service.post(s.user, text, nil)
			return doneMsg{posted: posted}, err
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
		posted, err := s.service.post(s.user, text, &replyTo)
		return doneMsg{posted: posted}, err
	})
}

func (s Screen) delete() tea.Cmd {
	id := s.details.Post.Id
	return s.request("deleting...", func() (tea.Msg, error) {
		return deletedMsg{}, s.service.delete(s.user, id)
	})
}

// reload loads the current mode again. In list mode the selected post stays selected once the list arrives.
func (s *Screen) reload() tea.Cmd {
	if s.mode == modePost {
		return s.loadDetails(s.details.Post.Descriptor())
	}

	if post, ok := s.selected(); ok {
		s.pending = &post.Id
	}

	return s.loadList(nil)
}

// shown returns posts shown as list items in order, and the list index of the first one.
func (s Screen) shown() ([]sdk.CommunityPost, int) {
	first := s.fieldIndex() + 1
	if s.attaching {
		first++
	}

	if s.mode == modeList {
		return s.posts, first
	}

	var posts []sdk.CommunityPost
	for _, reply := range s.replies {
		posts = append(posts, reply.Posts()...)
	}

	return posts, first
}

// indexOfPost returns the list index of post with id, or -1 when it isn't shown.
func (s Screen) indexOfPost(id sdk.CommunityPostId) int {
	posts, first := s.shown()
	for i, post := range posts {
		if post.Id == id {
			return first + i
		}
	}

	return -1
}

// selected returns the post under the cursor, when the cursor is on a post.
func (s Screen) selected() (sdk.CommunityPost, bool) {
	posts, first := s.shown()
	if i := s.content.list.Cursor() - first; i >= 0 && i < len(posts) {
		return posts[i], true
	}

	return sdk.CommunityPost{}, false
}

// selectPending selects the pending post when it is shown, keeping the scroll offset.
func (s *Screen) selectPending() {
	if s.pending == nil {
		return
	}

	if i := s.indexOfPost(*s.pending); i >= 0 {
		_, offset := s.content.list.Position()
		s.content.list.SetPosition(i, offset)
	}

	s.pending = nil
}

// known returns post with id from what the screen already has loaded.
func (s Screen) known(id sdk.CommunityPostId) (sdk.CommunityPost, bool) {
	candidates := append([]sdk.CommunityPost{}, s.posts...)
	for _, reply := range s.replies {
		candidates = append(candidates, reply.Posts()...)
	}

	if s.details != nil {
		candidates = append(candidates, s.details.Post)
		candidates = append(candidates, s.details.Upstream...)
	}

	for _, post := range candidates {
		if post.Id == id {
			return post, true
		}
	}

	return sdk.CommunityPost{}, false
}

// open shows post right away. Its replies and parents arrive with the details. It returns the sequence that frees
// pictures of the previous post, for tea.Raw.
func (s *Screen) open(post sdk.CommunityPost) string {
	freed := s.dropPictures(post.Text)
	s.mode = modePost
	s.picking, s.editing, s.confirmDelete = false, false, false
	s.details = &sdk.CommunityPostDetails{Post: post}
	s.replies, s.repliesNext = nil, nil
	s.content.field.Raw().SetValue("")
	s.content.list.Reset(s.items()...)
	return freed
}

func (s Screen) loadPictures(post sdk.CommunityPost) tea.Cmd {
	if post.Deleted() {
		return nil
	}

	var cmds []tea.Cmd
	for _, match := range imagePattern.FindAllStringSubmatch(post.Text.Value(), -1) {
		url := match[1]
		if _, ok := s.pictures[url]; ok {
			continue
		}

		s.pictures[url] = &picture{}
		width, rows := s.textWidth(), s.imageRows()
		cmds = append(cmds, func() tea.Msg {
			msg := imageMsg{url: url}
			msg.img, _ = s.service.image(url)
			if msg.img != nil && s.graphics != nil {
				msg.cols, msg.rows = ui.Fit(msg.img, width, rows)
				msg.id, msg.upload, _ = s.graphics.Upload(msg.img, msg.cols, msg.rows)
			}

			return router.TargetMsg{Type: screen.TypeCommunity, Inner: msg}
		})
	}

	return tea.Batch(cmds...)
}

func (s Screen) imageRows() int {
	// the opened post and its author line fit into the list clipping height, which is half of the screen
	return max(s.height/2-3, 4)
}

// place resizes uploaded picture to fit the current screen size. It returns the sequence for tea.Raw.
func (s Screen) place(p *picture) string {
	if p.id == 0 {
		return ""
	}

	cols, rows := ui.Fit(p.img, s.textWidth(), s.imageRows())
	if cols == p.cols && rows == p.rows {
		return ""
	}

	p.cols, p.rows = cols, rows
	return s.graphics.Place(p.id, cols, rows)
}

// dropPictures forgets pictures not used by text. It returns the sequence that frees their uploads, for tea.Raw.
func (s Screen) dropPictures(text *sdk.CommunityPostText) string {
	keep := make(map[string]bool)
	if text != nil {
		for _, match := range imagePattern.FindAllStringSubmatch(text.Value(), -1) {
			keep[match[1]] = true
		}
	}

	var freed strings.Builder
	for url, p := range s.pictures {
		if keep[url] {
			continue
		}

		if p.id != 0 {
			freed.WriteString(s.graphics.Delete(p.id))
		}
		delete(s.pictures, url)
	}

	clear(s.rendered)
	return freed.String()
}

// raw returns command writing seq to the terminal, nil when there is nothing to write.
func raw(seq string) tea.Cmd {
	if seq == "" {
		return nil
	}

	return tea.Raw(seq)
}

// stopAttaching hides path prompt and moves cursor back to the text field.
func (s *Screen) stopAttaching() {
	s.attaching = false
	s.content.prompt.Update(ui.UnfocusMsg{})
	s.content.list.Reset(s.items()...)
	s.content.list.Select(s.fieldIndex())
}

func (s Screen) owns(post sdk.CommunityPost) bool {
	return s.user != nil && post.Owner != nil && post.Owner.Id == s.user.Id
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.ActionMsg:
		action := ui.Dispatch(s.actions(), msg)
		// any other key drops pending delete confirmation
		if _, ok := action.(deleteMsg); !ok {
			s.confirmDelete = false
		}

		if action == nil {
			return s, nil
		}

		return s, screen.Send(action)
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.content.list.Set(s.items()...)
		var placed strings.Builder
		for _, p := range s.pictures {
			placed.WriteString(s.place(p))
		}

		return s, raw(placed.String())
	case auth.LogoutMsg:
		freed := s.dropPictures(nil)
		s.user = nil
		s.mode = modeList
		s.from = ""
		s.stack = nil
		s.picking = false
		s.posts, s.next = nil, nil
		s.details, s.replies, s.repliesNext = nil, nil, nil
		s.editing, s.confirmDelete, s.loadingMore, s.attaching = false, false, false, false
		s.content.field.Raw().SetValue("")
		s.content.status.Set("")
		s.content.list.Reset(s.items()...)
		return s, raw(freed)
	case auth.LoginMsg:
		s.user = msg.User
		s.mode = modeList
		return s, s.loadList(nil)
	case refreshMsg:
		return s, s.reload()
	case moreMsg:
		if s.loadingMore || !s.hasMore() {
			return s, nil
		}

		s.loadingMore = true
		if s.mode == modePost {
			return s, s.loadReplies()
		}

		return s, s.loadList(s.next)
	case OpenMsg:
		// a post opened from inside another post keeps the origin of the first one
		if s.mode == modeList || msg.From != "" {
			s.from = msg.From
		}

		switch {
		case s.mode == modeList:
			s.listCursor, s.listOffset = s.content.list.Position()
			s.stack = nil
		case msg.From != "":
			s.stack = nil
		default:
			cursor, offset := s.content.list.Position()
			s.stack = append(s.stack, postView{s.details, s.replies, s.repliesNext, cursor, offset})
		}

		if post, ok := s.known(msg.Post.Id); ok {
			freed := s.open(post)
			return s, tea.Batch(raw(freed), s.loadPictures(post), s.loadDetails(msg.Post))
		}

		return s, s.loadDetails(msg.Post)
	case pickMsg:
		s.picking = true
		s.content.list.Reset(s.items()...)
		return s, nil
	case cancelPickMsg:
		s.picking = false
		s.content.list.Reset(s.items()...)
		return s, nil
	case openLinkMsg:
		s.picking = false
		s.content.list.Reset(s.items()...)
		s.content.status.Set(ui.MutedStyle.Render("opened " + msg.url))
		return s, func() tea.Msg {
			if err := browser.Open(msg.url); err != nil {
				return router.TargetMsg{Type: screen.TypeCommunity, Inner: failedMsg{err: err}}
			}

			return nil
		}
	case openImageMsg:
		s.picking = false
		s.content.list.Reset(s.items()...)
		return s, s.request("opening image...", func() (tea.Msg, error) {
			path, err := s.service.saveImage(msg.url)
			if err == nil {
				err = browser.OpenFile(path)
			}

			return openedMsg{}, err
		})
	case openedMsg:
		s.content.status.Set(ui.MutedStyle.Render("opened image"))
		return s, nil
	case backMsg:
		if s.mode == modeList {
			return s, nil
		}

		// back from a post opened in another post returns to that one and refreshes it in the background
		if n := len(s.stack); n > 0 {
			view := s.stack[n-1]
			s.stack = s.stack[:n-1]
			freed := s.dropPictures(view.details.Post.Text)
			s.picking, s.editing, s.confirmDelete = false, false, false
			s.details, s.replies, s.repliesNext = view.details, view.replies, view.repliesNext
			s.content.field.Raw().SetValue("")
			s.content.list.Reset(s.items()...)
			s.content.list.SetPosition(view.cursor, view.offset)
			return s, tea.Batch(raw(freed), s.loadPictures(view.details.Post), s.loadDetails(view.details.Post.Descriptor()))
		}

		s.mode = modeList
		s.picking = false
		s.details = nil
		freed := s.dropPictures(nil)
		s.editing = false
		s.content.field.Raw().SetValue("")
		s.content.status.Set("")
		s.content.list.Reset(s.items()...)
		s.content.list.SetPosition(s.listCursor, s.listOffset)
		if s.from != "" {
			from := s.from
			s.from = ""
			return s, tea.Batch(raw(freed), screen.Send(screen.ChangeMsg{NewType: from}))
		}

		return s, raw(freed)
	case editMsg:
		s.editing = true
		s.content.field.Raw().SetValue(s.details.Post.Text.Value())
		s.content.list.Reset(s.items()...)
		s.content.list.Select(s.fieldIndex())
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
		return s, nil
	case attachMsg:
		s.attaching = true
		s.content.prompt.Raw().SetValue("")
		s.content.list.Set(s.items()...)
		s.content.list.Select(s.fieldIndex() + 1)
		return s, screen.Send(ui.InsertMsg{})
	case attachDoneMsg:
		path := s.content.prompt.Value()
		s.stopAttaching()
		return s, tea.Batch(screen.Send(ui.NormalMsg{}), s.request("uploading image...", func() (tea.Msg, error) {
			url, err := s.service.upload(s.user, path)
			return attachedMsg{url: url}, err
		}))
	case ui.UnfocusMsg:
		// esc while typing the path cancels attaching
		if s.attaching {
			s.stopAttaching()
			return s, nil
		}
	case attachedMsg:
		text := s.content.field.Value()
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}

		s.content.field.Raw().SetValue(text + "![](" + msg.url + ")\n")
		s.content.status.Set("")
		return s, nil
	case cancelDeleteMsg:
		s.confirmDelete = false
		return s, nil
	case submitMsg:
		return s, s.submit()
	case listMsg:
		s.loadingMore = false
		s.next = msg.page.NextId
		s.content.status.Set("")
		if msg.append {
			s.posts = append(s.posts, msg.page.Data...)
			s.content.list.Set(s.items()...)
			return s, nil
		}

		s.posts = msg.page.Data
		if s.pending == nil {
			s.content.list.Reset(s.items()...)
			return s, nil
		}

		s.content.list.Set(s.items()...)
		s.selectPending()
		return s, nil
	case detailsMsg:
		// details of the post already open update it in place. The cursor and any draft stay.
		same := s.mode == modePost && s.details != nil && s.details.Post.Id == msg.details.Post.Id
		var freed string
		if !same {
			freed = s.dropPictures(msg.details.Post.Text)
			s.editing, s.confirmDelete = false, false
			s.content.field.Raw().SetValue("")
		}

		s.mode = modePost
		s.picking = false
		s.details = msg.details
		s.replies = msg.details.Replies.Data
		s.repliesNext = msg.details.Replies.NextId
		s.content.status.Set("")
		if same {
			s.content.list.Set(s.items()...)
		} else {
			s.content.list.Reset(s.items()...)
		}

		s.selectPending()
		return s, tea.Batch(raw(freed), s.loadPictures(msg.details.Post))
	case imageMsg:
		// drop uploads of pictures that were left before downloading or got downloaded twice
		if p, ok := s.pictures[msg.url]; !ok || p.done {
			if msg.id != 0 {
				return s, raw(s.graphics.Delete(msg.id))
			}

			return s, nil
		}

		p := &picture{img: msg.img, done: true, id: msg.id, cols: msg.cols, rows: msg.rows}
		s.pictures[msg.url] = p
		// rebuild items so the opened post shows the picture
		s.content.list.Set(s.items()...)
		return s, raw(msg.upload + s.place(p))
	case repliesMsg:
		s.loadingMore = false
		s.replies = append(s.replies, msg.page.Data...)
		s.repliesNext = msg.page.NextId
		s.content.status.Set("")
		s.content.list.Set(s.items()...)
		return s, nil
	case doneMsg:
		s.content.field.Raw().SetValue("")
		s.editing = false
		// a new reply gets selected once the post reloads
		if msg.posted != nil && s.mode == modePost {
			s.pending = &msg.posted.Id
		}

		return s, s.reload()
	case deletedMsg:
		s.confirmDelete = false
		if len(s.replies) > 0 {
			return s, s.reload()
		}

		if n := len(s.details.Upstream); n > 0 {
			return s, s.loadDetails(s.details.Upstream[n-1].Descriptor())
		}

		model, back := s.Update(backMsg{})
		return model, tea.Batch(back, model.(Screen).loadList(nil))
	case failedMsg:
		s.loadingMore = false
		s.confirmDelete = false
		s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
		return s, nil
	}

	_, cmd := s.content.list.Update(msg)

	// load the next page when the cursor gets close to the end
	if _, ok := msg.(ui.MoveMsg); ok && s.content.list.Cursor() >= s.content.list.Len()-3 {
		model, more := s.Update(moreMsg{})
		return model, tea.Batch(cmd, more)
	}

	return s, cmd
}

func (s Screen) hasMore() bool {
	if s.mode == modePost {
		return s.repliesNext != nil
	}

	return s.next != nil
}
