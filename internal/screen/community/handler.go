package community

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/friendly-social-ai/cli/internal/browser"
	"github.com/friendly-social-ai/cli/internal/router"
	"github.com/friendly-social-ai/cli/internal/screen"
	"github.com/friendly-social-ai/cli/internal/screen/auth"
	"github.com/friendly-social-ai/cli/internal/screen/user"
	"github.com/friendly-social-ai/cli/internal/ui"
	sdk "github.com/friendly-social-ai/golang-sdk"
)

type mode int

const (
	modeList mode = iota
	modePost
)

// OpenMsg asks community screen to open post. From is the screen that asked, empty for the community screen itself.
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
	cancelAttachMsg  struct{}
	completePathMsg  struct{}
	pasteImageMsg    struct{}
	cancelDeleteMsg  struct{}
	submitMsg        struct{}
	refreshMsg       struct{}
	moreMsg          struct{}
	backMsg          struct{}
	topMsg           struct{}
	editMsg          struct{}
	deleteMsg        struct{}
	composeMsg       struct{}
	closeMsg         struct{}
	previewMsg       struct{}
	editorMsg        struct{}
	menuMsg          struct{}
	closeMenuMsg     struct{}
	discardMsg       struct{}
	cancelDiscardMsg struct{}
	filterMsg        struct{}
	filterDoneMsg    struct{}
	clearFilterMsg   struct{}
	authorMsg        struct{ owner sdk.UserDetails }
	completeMsg      struct{}
	hideEmojiMsg     struct{}
	// chooseMsg moves the chosen shortcode suggestion by step
	chooseMsg struct{ step int }
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
	// editedMsg reports that the external editor exited. path is the file holding the draft.
	editedMsg struct {
		path string
		err  error
	}
	// deletedMsg reports that post id was deleted.
	deletedMsg struct{ id sdk.CommunityPostId }
	// attachedMsg reports the upload of image n of the draft. prompt marks an upload started from the path prompt.
	attachedMsg struct {
		n      int
		url    string
		err    error
		prompt bool
	}
	imageMsg struct {
		url        string
		img        image.Image
		id         uint32
		cols, rows int
		upload     string
		// drawing is the half-block drawing of img when there are no terminal graphics, kept in rendered under key
		drawing, key string
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

	// composing shows the text field above the list. editing is the post it edits, nil for a new post or reply.
	// replyTo is the post a reply goes to in post mode.
	composing bool
	editing   *sdk.CommunityPostId
	replyTo   sdk.CommunityPost
	// previewing shows the draft rendered as markdown in place of the text field, from line previewOffset
	previewing    bool
	previewOffset int
	// menu shows the composer actions with single keys in place of typing
	menu bool
	// confirmDiscard asks to press the key again before the draft is gone
	confirmDiscard bool
	// composeOffset is the list scroll before the composer took room from it, restored when it closes
	composeOffset int
	// suggestion is the chosen shortcode suggestion or path completion. hidden is the shortcode esc hid suggestions for.
	suggestion int
	hidden     string
	// images holds markdown of draft images by the number of their [image N] token, "" while uploading. nextImage
	// numbers the next one, and uploads counts uploads in flight.
	images    map[int]string
	nextImage int
	uploads   int
	// files are completions of the path typed into the attach prompt, and fileInfo describes the file it points to.
	// The prompt starts in attachDir, the directory of the last attached file.
	files     []string
	fileInfo  string
	attachDir string

	confirmDelete bool
	loadingMore   bool
	attaching     bool
	picking       bool

	// listCursor and listOffset keep the list position while a post is open
	listCursor, listOffset int
	// pending is the post to select once the next load arrives
	pending *sdk.CommunityPostId
	// stack keeps posts the user opened replies from, so going up to one returns to its position
	stack []postView

	content struct {
		status *ui.Status
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
		service:   service,
		graphics:  graphics,
		markdown:  ui.NewMarkdown(),
		pictures:  make(map[string]*picture),
		rendered:  make(map[string]string),
		images:    make(map[int]string),
		attachDir: "~/",
	}

	input := textarea.New()
	input.Prompt = ""
	input.ShowLineNumbers = false
	input.CharLimit = 4096
	input.SetHeight(3)

	result.content.status = ui.NewStatus()
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

	s.content.status.Busy(status)
	return func() tea.Msg {
		msg, err := fn()
		if err != nil {
			msg = failedMsg{err: err}
		}

		return router.TargetMsg{Type: screen.TypeCommunity, Inner: msg}
	}
}

func (s Screen) loadList(cursor *sdk.CursorId) tea.Cmd {
	return s.request("loading posts", func() (tea.Msg, error) {
		page, err := s.service.list(s.user, cursor)
		return listMsg{page: page, append: cursor != nil}, err
	})
}

func (s Screen) loadDetails(post sdk.CommunityPostDescriptor) tea.Cmd {
	return s.request("loading post", func() (tea.Msg, error) {
		details, err := s.service.details(s.user, post)
		return detailsMsg{details: details}, err
	})
}

func (s Screen) loadReplies() tea.Cmd {
	post, cursor := s.details.Post.Descriptor(), s.repliesNext
	return s.request("loading replies", func() (tea.Msg, error) {
		page, err := s.service.replies(s.user, post, cursor)
		return repliesMsg{page: page}, err
	})
}

func (s Screen) submit() tea.Cmd {
	text := s.draft()

	if s.mode == modeList {
		return s.request("posting", func() (tea.Msg, error) {
			posted, err := s.service.post(s.user, text, nil)
			return doneMsg{posted: posted}, err
		})
	}

	if id := s.editing; id != nil {
		return s.request("saving", func() (tea.Msg, error) {
			return doneMsg{}, s.service.edit(s.user, *id, text)
		})
	}

	replyTo := s.replyTo.Descriptor()
	return s.request("replying", func() (tea.Msg, error) {
		posted, err := s.service.post(s.user, text, &replyTo)
		return doneMsg{posted: posted}, err
	})
}

func (s Screen) delete(id sdk.CommunityPostId) tea.Cmd {
	return s.request("deleting", func() (tea.Msg, error) {
		return deletedMsg{id: id}, s.service.delete(s.user, id)
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
	// every post matches an empty query, and matching runs regexes over each post
	if s.content.filter.Query() == "" {
		return s.posts
	}

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

// open shows post right away. upstream holds its parents when they are known, or nil. Its replies, and its parents
// when upstream is nil, arrive with the details. It returns the sequence that frees pictures of the previous post,
// for tea.Raw.
func (s *Screen) open(post sdk.CommunityPost, upstream []sdk.CommunityPost) string {
	freed := s.dropPictures(post.Text)
	s.mode = modePost
	s.closeComposer()
	s.picking, s.confirmDelete = false, false
	s.details = &sdk.CommunityPostDetails{Post: post, Upstream: upstream}
	s.replies, s.repliesNext = nil, nil
	s.content.field.Raw().SetValue("")
	s.content.list.Reset(s.items()...)
	s.content.list.SetPosition(s.openedIndex(), 0)
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
			// draw or upload the image here, off the update loop, since both take milliseconds
			switch {
			case msg.img != nil && s.graphics != nil:
				msg.cols, msg.rows = ui.Fit(msg.img, width, rows)
				msg.id, msg.upload, _ = s.graphics.Upload(msg.img, msg.cols, msg.rows)
			case msg.img != nil:
				msg.drawing, msg.key = ui.RenderImage(msg.img, width, rows), renderedKey(url, width, rows)
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

// stopAttaching hides path prompt and moves typing back to the text field.
func (s *Screen) stopAttaching() tea.Cmd {
	s.attaching = false
	s.content.prompt.Update(ui.UnfocusMsg{})
	_, cmd := s.content.field.Update(ui.FocusMsg{})
	return cmd
}

// attach puts an [image N] token at the cursor and runs upload, which returns the URL of the image. The token stands
// for the image until the upload ends, and goes away if it fails. prompt keeps the path prompt open until it succeeds.
func (s *Screen) attach(prompt bool, upload func() (string, error)) tea.Cmd {
	if s.user == nil {
		return nil
	}

	field := s.content.field.Raw()
	// numbering starts over once the draft has no images
	if s.uploads == 0 && !tokenPattern.MatchString(field.Value()) {
		clear(s.images)
		s.nextImage = 0
	}

	s.nextImage++
	n := s.nextImage
	token := fmt.Sprintf("[image %d]\n", n)
	if field.Column() > 0 {
		token = "\n" + token
	}

	if field.Length()+utf8.RuneCountInString(token) > field.CharLimit {
		return screen.Send(failedMsg{err: fmt.Errorf("draft too long for another image")})
	}

	field.InsertString(token)
	s.images[n] = ""
	s.uploads++
	return s.request("uploading image", func() (tea.Msg, error) {
		url, err := upload()
		return attachedMsg{n: n, url: url, err: err, prompt: prompt}, nil
	})
}

// removeToken removes the token of image n from the draft, with the line break after it, keeping the cursor in place.
func (s *Screen) removeToken(n int) {
	field := s.content.field.Raw()
	text := []rune(field.Value())
	cursor := field.Column()
	for _, line := range strings.Split(field.Value(), "\n")[:field.Line()] {
		cursor += utf8.RuneCountInString(line) + 1
	}

	pattern := regexp.MustCompile(fmt.Sprintf(`\[image %d\]\n?`, n))
	if !pattern.MatchString(field.Value()) {
		return
	}

	before, after := string(text[:cursor]), string(text[cursor:])
	// with the cursor inside the token, it ends up at the end
	if !pattern.MatchString(before) && !pattern.MatchString(after) {
		field.SetValue(pattern.ReplaceAllString(field.Value(), ""))
		return
	}

	field.SetValue(pattern.ReplaceAllString(after, ""))
	field.MoveToBegin()
	field.InsertString(pattern.ReplaceAllString(before, ""))
}

// draft returns the text of the draft with image tokens replaced by their markdown. Tokens of uploads in flight stay.
func (s Screen) draft() string {
	return tokenPattern.ReplaceAllStringFunc(s.content.field.Value(), func(token string) string {
		n, _ := strconv.Atoi(tokenPattern.FindStringSubmatch(token)[1])
		if markdown := s.images[n]; markdown != "" {
			return markdown
		}

		return token
	})
}

// setDraft fills the text field with text, showing its images as tokens.
func (s *Screen) setDraft(text string) {
	if s.uploads == 0 {
		clear(s.images)
		s.nextImage = 0
	}

	s.content.field.Raw().SetValue(imagePattern.ReplaceAllStringFunc(text, func(markdown string) string {
		s.nextImage++
		s.images[s.nextImage] = markdown
		return fmt.Sprintf("[image %d]", s.nextImage)
	}))
}

// promptPath returns the chosen completion of the attach prompt, or the typed path when there are no completions.
func (s Screen) promptPath() string {
	typed := s.content.prompt.Value()
	if len(s.files) == 0 {
		return typed
	}

	return typed[:strings.LastIndex(typed, "/")+1] + s.files[min(s.suggestion, len(s.files)-1)]
}

// completePath puts the chosen completion into the attach prompt.
func (s *Screen) completePath() {
	prompt := s.content.prompt.Raw()
	prompt.SetValue(s.promptPath())
	prompt.CursorEnd()
	s.refreshFiles()
}

// refreshFiles completes the path typed into the attach prompt and describes the file it points to.
func (s *Screen) refreshFiles() {
	s.files, s.suggestion = completions(s.content.prompt.Value()), 0
	s.describeFile()
}

// describeFile describes the file the attach prompt points to, or why it can't be attached.
func (s *Screen) describeFile() {
	info, err := describe(s.promptPath())
	switch {
	case err != nil:
		s.fileInfo = ui.DangerStyle.Render(err.Error())
	case info != "":
		s.fileInfo = ui.MutedStyle.Render(info)
	default:
		s.fileInfo = ""
	}
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

	// upstream already holds the parents of parent, so h can go further up before its details arrive
	freed := s.open(parent, upstream[:i])
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

// leave closes the opened post and goes back to the list. It selects the root of the thread if the list has it.
func (s *Screen) leave() tea.Cmd {
	root := s.details.Post.Id
	if len(s.details.Upstream) > 0 {
		root = s.details.Upstream[0].Id
	}

	s.mode = modeList
	s.picking = false
	s.details = nil
	freed := s.dropPictures(nil)
	s.closeComposer()
	s.content.field.Raw().SetValue("")
	s.content.status.Set("")
	s.content.list.Reset(s.items()...)
	s.content.list.SetPosition(s.listCursor, s.listOffset)
	s.pending = &root
	s.selectPending()
	return raw(freed)
}

// scrollPreview moves the preview by msg, a move, scroll, jump or wheel turn. It reports whether msg was one of them.
func (s *Screen) scrollPreview(msg tea.Msg) bool {
	last := max(lipgloss.Height(s.body(s.draft()))-s.previewRows(), 0)
	var direction ui.Direction
	step := 1
	switch msg := msg.(type) {
	case ui.MoveMsg:
		direction = msg.Direction
	case ui.WheelMsg:
		direction = msg.Direction
	case ui.ScrollMsg:
		direction, step = msg.Direction, max(s.previewRows()/2, 1)
	case ui.JumpMsg:
		direction, step = msg.Direction, last
	default:
		return false
	}

	if direction == ui.DirectionUp {
		step = -step
	}

	s.previewOffset = max(min(s.previewOffset+step, last), 0)
	return true
}

// openComposer shows the composer, and typing goes to its text field.
func (s *Screen) openComposer() {
	s.composing = true
	_, s.composeOffset = s.content.list.Position()
}

// openEditor writes the draft to a file and opens it in $VISUAL or $EDITOR, or in vi when neither is set.
func (s Screen) openEditor() tea.Cmd {
	f, err := os.CreateTemp("", "friendly-*.md")
	if err != nil {
		return screen.Send(failedMsg{err: fmt.Errorf("failed to create draft file: %w", err)})
	}

	_, err = f.WriteString(s.draft())
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		os.Remove(f.Name()) //nolint:errcheck
		return screen.Send(failedMsg{err: fmt.Errorf("failed to write draft file: %w", err)})
	}

	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}

	if editor == "" {
		editor = "vi"
	}

	// the shell splits editors with arguments, like code --wait
	cmd := exec.Command("sh", "-c", editor+` "$1"`, "sh", f.Name())
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			err = fmt.Errorf("editor %q failed: %w", editor, err)
		}

		return router.TargetMsg{Type: screen.TypeCommunity, Inner: editedMsg{path: f.Name(), err: err}}
	})
}

// closeComposer hides the composer. Its draft stays for the next one, except for an edit.
func (s *Screen) closeComposer() {
	s.stopAttaching()
	if s.composing {
		cursor, _ := s.content.list.Position()
		s.content.list.SetPosition(cursor, s.composeOffset)
	}

	s.composing, s.previewing, s.menu, s.confirmDiscard = false, false, false, false
	s.suggestion, s.hidden = 0, ""
	s.content.field.Update(ui.UnfocusMsg{})
	if s.editing != nil {
		s.editing = nil
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

		// a chosen action closes the menu
		s.menu = false
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
		switch {
		case s.mode == modeList:
			s.listCursor, s.listOffset = s.content.list.Position()
			s.stack = nil
		case msg.From != "":
			// a post opened from another screen starts a new thread
			s.stack = nil
		default:
			cursor, offset := s.content.list.Position()
			s.stack = append(s.stack, postView{s.details, s.replies, s.repliesNext, cursor, offset})
		}

		if post, ok := s.known(msg.Post.Id); ok {
			freed := s.open(post, nil)
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
		return s, s.request("opening image", func() (tea.Msg, error) {
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

		// h goes up the thread. Until details arrive the parent is unknown, so h returns to the post the user came from.
		if n := len(s.details.Upstream); n > 0 {
			return s, s.up(n - 1)
		}

		if len(s.stack) > 0 {
			return s, s.restore()
		}

		return s, s.leave()
	case topMsg:
		if s.mode == modeList {
			return s, nil
		}

		s.stack = nil
		return s, s.leave()
	case composeMsg:
		// a reply goes to the selected post. Every item of a thread is a post, so the cursor is always on one.
		if s.mode == modePost {
			s.replyTo, _ = s.cursorPost()
		}

		s.openComposer()
		return s, nil
	case upMsg:
		return s, s.up(msg.index)
	case copyMsg:
		return s, tea.Batch(tea.SetClipboard(msg.text), s.notice("copied "+msg.what))
	case authorMsg:
		return s, tea.Batch(
			screen.Send(router.TargetMsg{Type: screen.TypeUser, Inner: user.OpenMsg{Person: msg.owner, From: screen.TypeCommunity}}),
			screen.Send(screen.ChangeMsg{NewType: screen.TypeUser}))
	case editMsg:
		post, _ := s.cursorPost()
		s.editing = &post.Id
		s.setDraft(post.Text.Value())
		s.openComposer()
		return s, nil
	case closeMsg:
		s.closeComposer()
		return s, nil
	case filterMsg:
		s.content.filter.Start()
		return s, nil
	case filterDoneMsg:
		s.content.filter.Stop()
		return s, nil
	case clearFilterMsg:
		s.content.filter.Clear()
		s.content.list.Reset(s.items()...)
		return s, nil
	case menuMsg:
		s.menu = true
		return s, nil
	case closeMenuMsg:
		// the action already closed the menu
		return s, nil
	case completeMsg:
		found := s.suggestions()
		// backspaces remove the typed name after its colon, so the cursor ends up after the inserted shortcode
		for range utf8.RuneCountInString(s.shortcodeQuery()) {
			s.content.field.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		}

		s.content.field.Raw().InsertString(found[min(s.suggestion, len(found)-1)].Name + ":")
		s.suggestion = 0
		return s, nil
	case chooseMsg:
		if s.attaching {
			n := len(s.files)
			s.suggestion = (s.suggestion + msg.step + n) % n
			s.describeFile()
			return s, nil
		}

		n := len(s.suggestions())
		s.suggestion = (s.suggestion + msg.step + n) % n
		return s, nil
	case hideEmojiMsg:
		s.hidden = s.shortcodeQuery()
		return s, nil
	case editorMsg:
		return s, s.openEditor()
	case editedMsg:
		data, err := os.ReadFile(msg.path)
		os.Remove(msg.path) //nolint:errcheck
		if msg.err != nil {
			err = msg.err
		}

		text := strings.TrimSuffix(string(data), "\n")
		if n, limit := utf8.RuneCountInString(text), s.content.field.Raw().CharLimit; err == nil && n > limit {
			err = fmt.Errorf("draft too long, %d / %d", n, limit)
		}

		if err != nil {
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(err)))
			return s, nil
		}

		// an empty file leaves the draft as it was. The menu discards it with x.
		if text != "" {
			s.setDraft(text)
		}

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
		s.previewing, s.previewOffset = !s.previewing, 0
		if s.previewing {
			return s, s.loadImages(s.draft())
		}

		return s, nil
	case deleteMsg:
		if s.confirmDelete {
			post, _ := s.cursorPost()
			return s, s.delete(post.Id)
		}

		s.confirmDelete = true
		return s, nil
	case attachMsg:
		s.attaching = true
		prompt := s.content.prompt.Raw()
		prompt.SetValue(s.attachDir)
		prompt.CursorEnd()
		s.refreshFiles()
		s.content.field.Update(ui.UnfocusMsg{})
		_, cmd := s.content.prompt.Update(ui.FocusMsg{})
		return s, cmd
	case completePathMsg:
		s.completePath()
		return s, nil
	case attachDoneMsg:
		// enter on a directory opens it
		path := s.promptPath()
		if strings.HasSuffix(path, "/") {
			s.completePath()
			return s, nil
		}

		if dir := path[:strings.LastIndex(path, "/")+1]; dir != "" {
			s.attachDir = dir
		}

		return s, s.attach(true, func() (string, error) {
			return s.service.upload(s.user, path)
		})
	case cancelAttachMsg:
		return s, s.stopAttaching()
	case pasteImageMsg:
		return s, s.attach(false, func() (string, error) {
			return s.service.uploadClipboard(s.user)
		})
	case tea.PasteMsg:
		// image files dropped onto the composer upload in place of pasting their paths
		if !s.composing || s.previewing {
			break
		}

		paths := droppedImages(msg.Content)
		if paths == nil {
			break
		}

		var cmds []tea.Cmd
		if s.attaching {
			cmds = append(cmds, s.stopAttaching())
		}

		for _, path := range paths {
			cmds = append(cmds, s.attach(false, func() (string, error) {
				return s.service.upload(s.user, path)
			}))
		}

		return s, tea.Batch(cmds...)
	case ui.ClickMsg:
		// while the composer is open, a click keeps typing in it instead of selecting a post
		if s.composing {
			return s, nil
		}
	case ui.UnfocusMsg:
		// a click while typing the path cancels attaching
		if s.attaching {
			s.stopAttaching()
			return s, nil
		}
	case attachedMsg:
		s.uploads--
		if s.uploads == 0 {
			s.content.status.Set("")
		}

		// a failed upload leaves the prompt open with its path, to fix it or try again
		if msg.err != nil {
			s.removeToken(msg.n)
			delete(s.images, msg.n)
			s.content.status.Set(ui.DangerStyle.Render(screen.ErrorText(msg.err)))
			return s, nil
		}

		s.images[msg.n] = "![](" + msg.url + ")"
		if msg.prompt && s.attaching {
			return s, s.stopAttaching()
		}

		return s, nil
	case cancelDeleteMsg:
		s.confirmDelete = false
		return s, nil
	case submitMsg:
		if s.uploads > 0 {
			return s, s.notice("wait for images to upload")
		}

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
		if msg.drawing != "" {
			s.rendered[msg.key] = msg.drawing
		}

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

		return s, s.reload()
	case deletedMsg:
		s.confirmDelete = false
		// a deleted reply or parent stays in the thread marked as deleted, and so does a deleted post with replies
		if msg.id != s.details.Post.Id || len(s.replies) > 0 {
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
		// the preview has no field to type in, so moves and scrolls scroll a long draft
		if s.previewing && s.scrollPreview(msg) {
			return s, nil
		}

		if s.attaching {
			typed := s.content.prompt.Value()
			_, cmd := s.content.prompt.Update(msg)
			if s.content.prompt.Value() != typed {
				s.refreshFiles()
			}

			return s, cmd
		}

		// typing a different shortcode chooses its first suggestion again
		typed := s.shortcodeQuery()
		_, cmd := s.content.field.Update(msg)
		if s.shortcodeQuery() != typed {
			s.suggestion = 0
		}

		return s, cmd
	}

	// moving to another post cancels the delete confirmation, which is for the selected post
	if ui.Moves(msg) {
		s.confirmDelete = false
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
