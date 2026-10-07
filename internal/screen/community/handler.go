package community

import (
	"image"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/friendly-social/cli/internal/browser"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/screen/auth"
	"github.com/friendly-social/cli/internal/screen/user"
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
	attachMsg        struct{}
	pickMsg          struct{}
	cancelPickMsg    struct{}
	openLinkMsg      struct{ url string }
	openImageMsg     struct{ url string }
	attachDoneMsg    struct{}
	cancelDeleteMsg  struct{}
	submitMsg        struct{}
	refreshMsg       struct{}
	moreMsg          struct{}
	backMsg          struct{}
	editMsg          struct{}
	deleteMsg        struct{}
	composeMsg       struct{}
	closeMsg         struct{}
	previewMsg       struct{}
	discardMsg       struct{}
	cancelDiscardMsg struct{}
	filterMsg        struct{}
	filterDoneMsg    struct{}
	clearFilterMsg   struct{}
	authorMsg        struct{ owner sdk.UserDetails }
	// copyMsg puts text in the clipboard. what names the copied thing in the notice.
	copyMsg struct{ text, what string }
	// upMsg opens parent index of the opened post, counted from the top of the thread
	upMsg struct{ index int }
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
	// clearNoticeMsg clears the status when it still shows notice text
	clearNoticeMsg struct{ text string }
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

	// composing shows the text field above the list, and editing makes it edit the opened post
	composing bool
	editing   bool
	// previewing shows the draft rendered as markdown in place of the text field
	previewing bool
	// confirmDiscard asks to press the key again before the draft is gone
	confirmDiscard bool
	// composeOffset is the list scroll before the composer took room from it, restored when it closes
	composeOffset int

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
	// stack keeps posts the user opened replies from, so going up to one returns to its position
	stack []postView

	content struct {
		status *ui.Label
		filter *ui.Filter
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
	result.content.filter = ui.NewFilter()
	result.content.field = ui.NewTextArea(input)

	prompt := textinput.New()
	prompt.Prompt = ""
	prompt.Placeholder = "Path to a png, jpeg or gif, or drop a file here"
	result.content.prompt = ui.NewField(prompt)
	result.content.list = ui.NewList()
	result.content.list.SetGap(1)

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

	if post, ok := s.selected(); ok && s.pending == nil {
		s.pending = &post.Id
	}

	return s.loadList(nil)
}

// shown returns posts shown as list items in order, and the list index of the first one, which follows the parents
// and the opened post in post mode.
func (s Screen) shown() ([]sdk.CommunityPost, int) {
	if s.mode == modeList {
		return s.listed(), 0
	}

	var posts []sdk.CommunityPost
	for _, reply := range s.replies {
		posts = append(posts, reply.Posts()...)
	}

	return posts, s.openedIndex() + 1
}

// listed returns posts of the list that match the filter by author or text.
func (s Screen) listed() []sdk.CommunityPost {
	var posts []sdk.CommunityPost
	for _, post := range s.posts {
		author, _ := metaParts(post)
		if s.content.filter.Match(author + " " + FirstLine(post)) {
			posts = append(posts, post)
		}
	}

	return posts
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

// cursorPost returns the post under the cursor: a parent, the opened post, a reply or a post of the list.
func (s Screen) cursorPost() (sdk.CommunityPost, bool) {
	if cursor := s.content.list.Cursor(); s.mode == modePost {
		switch {
		case cursor < s.openedIndex():
			return s.details.Upstream[cursor], true
		case cursor == s.openedIndex():
			return s.details.Post, true
		}
	}

	return s.selected()
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
	s.closeComposer()
	s.picking, s.confirmDelete = false, false
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

	return s.loadImages(post.Text.Value())
}

// loadImages downloads images of text that aren't loaded yet.
func (s Screen) loadImages(text string) tea.Cmd {
	var cmds []tea.Cmd
	for _, match := range imagePattern.FindAllStringSubmatch(text, -1) {
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

// notice shows text in the status for a few seconds.
func (s Screen) notice(text string) tea.Cmd {
	s.content.status.Set(ui.MutedStyle.Render(text))
	return tea.Tick(4*time.Second, func(time.Time) tea.Msg {
		return router.TargetMsg{Type: screen.TypeCommunity, Inner: clearNoticeMsg{text: text}}
	})
}

// raw returns command writing seq to the terminal, nil when there is nothing to write.
func raw(seq string) tea.Cmd {
	if seq == "" {
		return nil
	}

	return tea.Raw(seq)
}

// stopPicking hides the links of the opened post and selects it again.
func (s *Screen) stopPicking() {
	s.picking = false
	s.content.list.Reset(s.items()...)
	s.content.list.Select(s.openedIndex())
}

// stopAttaching hides path prompt.
func (s *Screen) stopAttaching() {
	s.attaching = false
	s.content.prompt.Update(ui.UnfocusMsg{})
}

// up opens parent i of the opened post. If the user came from that parent, it returns from the stack at its saved
// position. Otherwise it opens with the next post down the thread selected.
func (s *Screen) up(i int) tea.Cmd {
	upstream := s.details.Upstream
	depth := func(view postView) int {
		for j, post := range upstream {
			if post.Id == view.details.Post.Id {
				return j
			}
		}

		return -1
	}

	// drop stack entries below the parent, since the stack holds posts on the way down
	for m := len(s.stack); m > 0; m = len(s.stack) {
		if d := depth(s.stack[m-1]); d >= 0 && d <= i {
			break
		}

		s.stack = s.stack[:m-1]
	}

	if m := len(s.stack); m > 0 && depth(s.stack[m-1]) == i {
		return s.restore()
	}

	parent, child := upstream[i], s.details.Post.Id
	if i+1 < len(upstream) {
		child = upstream[i+1].Id
	}

	freed := s.open(parent)
	s.pending = &child
	return tea.Batch(raw(freed), s.loadPictures(parent), s.loadDetails(parent.Descriptor()))
}

// restore brings back the post on top of the stack at its position, and refreshes it in the background.
func (s *Screen) restore() tea.Cmd {
	view := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	freed := s.dropPictures(view.details.Post.Text)
	s.closeComposer()
	s.picking, s.confirmDelete = false, false
	s.details, s.replies, s.repliesNext = view.details, view.replies, view.repliesNext
	s.content.field.Raw().SetValue("")
	s.content.list.Reset(s.items()...)
	s.content.list.SetPosition(view.cursor, view.offset)
	return tea.Batch(raw(freed), s.loadPictures(view.details.Post), s.loadDetails(view.details.Post.Descriptor()))
}

// openComposer shows the composer and starts typing in it.
func (s *Screen) openComposer() tea.Cmd {
	s.composing = true
	_, s.composeOffset = s.content.list.Position()
	return screen.Send(ui.InsertMsg{})
}

// closeComposer hides the composer. Its draft stays for the next one, except for an edit.
func (s *Screen) closeComposer() {
	s.stopAttaching()
	if s.composing {
		cursor, _ := s.content.list.Position()
		s.content.list.SetPosition(cursor, s.composeOffset)
	}

	s.composing, s.previewing, s.confirmDiscard = false, false, false
	s.content.field.Update(ui.UnfocusMsg{})
	if s.editing {
		s.editing = false
		s.content.field.Raw().SetValue("")
	}
}

func (s Screen) owns(post sdk.CommunityPost) bool {
	return s.user != nil && post.Owner != nil && post.Owner.Id == s.user.Id
}

func (s Screen) Update(msg tea.Msg) (screen.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.ActionMsg:
		action := ui.Dispatch(s.actions(), msg)
		// any other key drops pending confirmations
		if _, ok := action.(deleteMsg); !ok {
			s.confirmDelete = false
		}

		if _, ok := action.(discardMsg); !ok {
			s.confirmDiscard = false
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
		s.closeComposer()
		s.confirmDelete, s.loadingMore = false, false
		s.content.field.Raw().SetValue("")
		s.content.status.Set("")
		s.content.filter.Clear()
		s.content.list.Reset(s.items()...)
		return s, raw(freed)
	case auth.LoginMsg:
		s.user = msg.User
		s.mode = modeList
		return s, s.loadList(nil)
	case screen.MinuteMsg:
		// rebuilt items show fresh relative times
		s.content.list.Set(s.items()...)
		return s, nil
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
		s.stopPicking()
		return s, nil
	case openLinkMsg:
		s.stopPicking()
		return s, tea.Batch(s.notice("opened "+msg.url), func() tea.Msg {
			if err := browser.Open(msg.url); err != nil {
				return router.TargetMsg{Type: screen.TypeCommunity, Inner: failedMsg{err: err}}
			}

			return nil
		})
	case openImageMsg:
		s.stopPicking()
		return s, s.request("opening image...", func() (tea.Msg, error) {
			path, err := s.service.saveImage(msg.url)
			if err == nil {
				err = browser.OpenFile(path)
			}

			return openedMsg{}, err
		})
	case openedMsg:
		return s, s.notice("opened image")
	case clearNoticeMsg:
		if s.content.status.Value() == ui.MutedStyle.Render(msg.text) {
			s.content.status.Set("")
		}

		return s, nil
	case backMsg:
		if s.mode == modeList {
			return s, nil
		}

		// esc goes up the thread. Until details arrive the parent is unknown, so the post the user came from is used.
		if n := len(s.details.Upstream); n > 0 {
			return s, s.up(n - 1)
		}

		if len(s.stack) > 0 {
			return s, s.restore()
		}

		// a top-level post goes back to where it was opened from
		s.mode = modeList
		s.picking = false
		s.details = nil
		freed := s.dropPictures(nil)
		s.closeComposer()
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
	case composeMsg:
		return s, s.openComposer()
	case upMsg:
		return s, s.up(msg.index)
	case copyMsg:
		return s, tea.Batch(tea.SetClipboard(msg.text), s.notice("copied "+msg.what))
	case authorMsg:
		return s, tea.Batch(
			screen.Send(router.TargetMsg{Type: screen.TypeUser, Inner: user.OpenMsg{Person: msg.owner, From: screen.TypeCommunity}}),
			screen.Send(screen.ChangeMsg{NewType: screen.TypeUser}))
	case editMsg:
		s.editing = true
		s.content.field.Raw().SetValue(s.details.Post.Text.Value())
		return s, s.openComposer()
	case closeMsg:
		s.closeComposer()
		return s, nil
	case filterMsg:
		s.content.filter.Start()
		return s, screen.Send(ui.InsertMsg{})
	case filterDoneMsg:
		return s, screen.Send(ui.NormalMsg{})
	case clearFilterMsg:
		s.content.filter.Clear()
		s.content.list.Reset(s.items()...)
		return s, nil
	case discardMsg:
		if s.confirmDiscard {
			s.confirmDiscard = false
			s.content.field.Raw().SetValue("")
			return s, nil
		}

		s.confirmDiscard = true
		return s, nil
	case cancelDiscardMsg:
		s.confirmDiscard = false
		return s, nil
	case previewMsg:
		s.previewing = !s.previewing
		if s.previewing {
			return s, s.loadImages(s.content.field.Value())
		}

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
		return s, screen.Send(ui.InsertMsg{})
	case attachDoneMsg:
		path := s.content.prompt.Value()
		s.stopAttaching()
		return s, tea.Batch(screen.Send(ui.NormalMsg{}), s.request("uploading image...", func() (tea.Msg, error) {
			url, err := s.service.upload(s.user, path)
			return attachedMsg{url: url}, err
		}))
	case ui.ClickMsg:
		// while the composer is open, a click starts typing in it instead of selecting a post
		if s.composing && !s.previewing {
			return s, screen.Send(ui.InsertMsg{})
		}
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
		// parents come first in the list, so a change in their number moves the other items
		shift := len(msg.details.Upstream)
		if same {
			shift -= len(s.details.Upstream)
		}

		var freed string
		if !same {
			freed = s.dropPictures(msg.details.Post.Text)
			s.closeComposer()
			s.confirmDelete = false
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

		if shift != 0 {
			cursor, offset := s.content.list.Position()
			s.content.list.SetPosition(cursor+shift, offset)
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
		s.closeComposer()
		// a new post or reply gets selected once it loads
		if msg.posted != nil {
			s.pending = &msg.posted.Id
		}

		return s, tea.Batch(screen.Send(ui.NormalMsg{}), s.reload())
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

	// typing the filter narrows the list as the query changes
	if s.content.filter.Typing() {
		changed, cmd := s.content.filter.Update(msg)
		if changed {
			s.content.list.Reset(s.items()...)
		}

		return s, cmd
	}

	// the open composer takes typing and focus, and the list stays where it is
	if s.composing {
		target := ui.Component(s.content.field)
		if s.attaching {
			target = s.content.prompt
		}

		_, cmd := target.Update(msg)
		return s, cmd
	}

	_, cmd := s.content.list.Update(msg)

	// load the next page when the cursor gets close to the end, or the wheel scrolls the end on screen
	if ui.Moves(msg) && (s.content.list.Cursor() >= s.content.list.Len()-3 || s.content.list.AtEnd()) {
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
