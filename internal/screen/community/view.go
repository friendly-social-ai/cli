package community

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social-ai/cli/internal/screen"
	"github.com/friendly-social-ai/cli/internal/ui"
	sdk "github.com/friendly-social-ai/golang-sdk"
)

var (
	// imagePattern matches markdown image and captures its URL.
	imagePattern = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)\)`)
	// inlineLinkPattern matches markdown link and captures its text.
	inlineLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\([^)\s]*\)`)
	// blockMarkPattern matches heading, quote and list marks starting a line.
	blockMarkPattern = regexp.MustCompile(`^(#{1,6}|>|[-*+]|\d+\.)\s+`)
	// inlineMarks removes bold and code marks.
	inlineMarks = strings.NewReplacer("**", "", "__", "", "`", "")
	// shortcodeQueryPattern matches a shortcode being typed at the end of text and captures its name. The colon starts
	// a word, so times like 10:30 and links don't count.
	shortcodeQueryPattern = regexp.MustCompile(`(?:^|\s):([A-Za-z0-9_+-]{2,})$`)
	// tokenPattern matches the [image N] token that stands for an image in the draft and captures its number.
	tokenPattern = regexp.MustCompile(`\[image (\d+)\]`)
)

// suggestionLimit is the number of shortcodes the composer suggests at once.
const suggestionLimit = 6

// items builds elements of the current mode. List mode shows posts. Post mode shows the parents from the top of the
// thread, then the opened post and its replies.
func (s Screen) items() []ui.Component {
	var items []ui.Component
	if s.picking {
		for _, l := range links(s.details.Post.Text.Value()) {
			title := ui.BoldStyle.Render(ansi.Truncate(l.label, s.textWidth(), "…"))
			if l.label != l.url {
				title += "\n" + ui.MutedStyle.Render(ansi.Truncate(l.url, s.textWidth(), "…"))
			}

			items = append(items, ui.NewButton(title, screen.Send(openLinkAction(l))))
		}

		return items
	}

	if s.mode == modeList {
		for _, post := range s.listed() {
			items = append(items, s.postButton(post, ""))
		}

		return items
	}

	for i, post := range s.details.Upstream {
		author, _ := metaParts(post)
		line := ansi.Truncate("↑ "+author+": "+FirstLine(post), s.textWidth(), "…")
		items = append(items, ui.NewButton(ui.MutedStyle.Render(line), screen.Send(upMsg{index: i})))
	}

	items = append(items, ui.NewLabel(s.opened()))
	for _, reply := range s.replies {
		for i, post := range reply.Posts() {
			indent := ""
			if i > 0 {
				indent = ui.MutedStyle.Render("│ ")
			}

			items = append(items, s.postButton(post, indent))
		}
	}

	return items
}

// actions builds keys available in the current state. While typing, only the keys that can't be text work.
func (s Screen) actions() []ui.Action {
	if s.attaching {
		var actions []ui.Action
		// enter is hidden while an upload runs
		if s.uploads == 0 {
			desc := "attach"
			if strings.HasSuffix(s.promptPath(), "/") {
				desc = "open"
			}

			actions = append(actions, ui.Action{Key: ui.Key("enter", desc), Msg: attachDoneMsg{}})
		}

		if len(s.files) > 0 {
			actions = append(actions,
				ui.Action{Key: ui.Key("tab", "complete"), Msg: completePathMsg{}},
				ui.Action{Key: ui.Key("down", "next", "ctrl+n"), Msg: chooseMsg{step: 1}},
				ui.Action{Key: ui.Key("up", "previous", "ctrl+p"), Msg: chooseMsg{step: -1}})
		}

		return append(actions, ui.Action{Key: ui.Key("esc", "cancel"), Msg: cancelAttachMsg{}})
	}

	if s.picking {
		l := links(s.details.Post.Text.Value())[s.content.list.Cursor()]
		return []ui.Action{
			{Key: ui.Key("l", "open", "enter")},
			{Key: ui.Key("y", "copy link"), Msg: copyMsg{text: l.url, what: "link"}},
			{Key: ui.Key("esc", "cancel"), Msg: cancelPickMsg{}},
		}
	}

	if s.content.filter.Typing() {
		return []ui.Action{{Key: ui.Key("enter", "done", "esc"), Msg: filterDoneMsg{}}}
	}

	if s.composing && s.confirmDiscard {
		return []ui.Action{
			{Key: ui.Key("x", "confirm discard"), Msg: discardMsg{}},
			{Key: ui.Key("esc", "cancel"), Msg: cancelDiscardMsg{}},
		}
	}

	if s.composing && s.menu {
		return s.menuActions()
	}

	if s.composing {
		post := ui.Action{Key: ui.Key("alt+enter", s.submitLabel()), Msg: submitMsg{}}
		if s.previewing {
			return []ui.Action{{Key: ui.Key("esc", "edit", "p"), Msg: previewMsg{}}, post}
		}

		var actions []ui.Action
		if found := s.suggestions(); len(found) > 0 {
			name := ":" + found[min(s.suggestion, len(found)-1)].Name + ":"
			actions = append(actions,
				ui.Action{Key: ui.Key("tab", "insert "+name, "enter"), Msg: completeMsg{}},
				ui.Action{Key: ui.Key("down", "next", "ctrl+n"), Msg: chooseMsg{step: 1}},
				ui.Action{Key: ui.Key("up", "previous", "ctrl+p"), Msg: chooseMsg{step: -1}},
				ui.Action{Key: ui.Key("esc", "hide"), Msg: hideEmojiMsg{}},
				post,
				ui.Action{Key: ui.Key("ctrl+o", "menu"), Msg: menuMsg{}})
			return actions
		}

		if s.content.field.Value() != "" {
			actions = append(actions, post)
		}

		return append(actions,
			ui.Action{Key: ui.Key("ctrl+o", "menu"), Msg: menuMsg{}},
			ui.Action{Key: ui.Key("esc", "close"), Msg: closeMsg{}})
	}

	var actions []ui.Action
	if s.content.list.Scrollable() {
		actions = append(actions, ui.Action{Key: ui.Key("ctrl+d/u", "scroll")})
	}

	switch cursor := s.content.list.Cursor(); {
	case s.mode == modePost && cursor < s.openedIndex():
		actions = append(actions, ui.Action{Key: ui.Key("l", "open", "enter")})
	case s.mode == modePost && cursor == s.openedIndex():
		if s.details.Post.Deleted() {
			break
		}

		// with the cursor on the opened post, a single link opens right away and several open a picker
		switch found := links(s.details.Post.Text.Value()); len(found) {
		case 0:
		case 1:
			desc := "open link"
			if found[0].image {
				desc = "open image"
			}

			actions = append(actions, ui.Action{Key: ui.Key("o", desc), Msg: openLinkAction(found[0])})
		default:
			actions = append(actions, ui.Action{Key: ui.Key("o", "links"), Msg: pickMsg{}})
		}
	case s.content.list.Len() > 0:
		actions = append(actions, ui.Action{Key: ui.Key("l", "open", "enter")})
	}

	if post, ok := s.cursorPost(); ok && !post.Deleted() {
		actions = append(actions, ui.Action{Key: ui.Key("y", "copy"), Msg: copyMsg{text: post.Text.Value(), what: "post"}})
		if post.Owner != nil && !s.owns(post) {
			actions = append(actions, ui.Action{Key: ui.Key("@", "author"), Msg: authorMsg{owner: *post.Owner}})
		}
	}

	refresh := ui.Action{Key: ui.Key("r", "refresh"), Msg: refreshMsg{}}
	if s.mode == modeList {
		actions = append(actions,
			ui.Action{Key: ui.Key("n", "new post"), Msg: composeMsg{}},
			ui.Action{Key: ui.Key("/", "filter"), Msg: filterMsg{}},
			refresh)
		if s.content.filter.Query() != "" {
			actions = append(actions, ui.Action{Key: ui.Key("esc", "clear"), Msg: clearFilterMsg{}})
		}

		return actions
	}

	if s.confirmDelete {
		return append(actions,
			ui.Action{Key: ui.Key("d", "confirm delete"), Msg: deleteMsg{}},
			ui.Action{Key: ui.Key("esc", "cancel"), Msg: cancelDeleteMsg{}})
	}

	actions = append(actions, ui.Action{Key: ui.Key("n", "reply"), Msg: composeMsg{}})
	if post, ok := s.cursorPost(); ok && s.owns(post) && !post.Deleted() {
		actions = append(actions,
			ui.Action{Key: ui.Key("e", "edit"), Msg: editMsg{}},
			ui.Action{Key: ui.Key("d", "delete"), Msg: deleteMsg{}})
	}

	// h goes up one level. From the top of the thread it goes to the list. Deeper in a thread, H goes straight to
	// the list.
	actions = append(actions, refresh)
	if len(s.details.Upstream) > 0 || len(s.stack) > 0 {
		return append(actions,
			ui.Action{Key: ui.Key("h", "to parent", "esc"), Msg: backMsg{}},
			ui.Action{Key: ui.Key("H", "to list"), Msg: topMsg{}})
	}

	return append(actions, ui.Action{Key: ui.Key("h", "to list", "esc"), Msg: backMsg{}})
}

// openLinkAction returns the message that opens l. Images open in the image viewer, other links in the browser.
func openLinkAction(l link) tea.Msg {
	if l.image {
		return openImageMsg{url: l.url}
	}

	return openLinkMsg{url: l.url}
}

func (s Screen) submitLabel() string {
	switch {
	case s.mode == modeList:
		return "post"
	case s.editing != nil:
		return "save"
	}

	return "reply"
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
}

// menuActions builds keys of the composer menu. Preview and discard need a draft.
func (s Screen) menuActions() []ui.Action {
	actions := []ui.Action{{Key: ui.Key("e", "editor"), Msg: editorMsg{}}}
	if s.content.field.Value() != "" {
		actions = append(actions, ui.Action{Key: ui.Key("p", "preview"), Msg: previewMsg{}})
	}

	actions = append(actions,
		ui.Action{Key: ui.Key("a", "attach image"), Msg: attachMsg{}},
		ui.Action{Key: ui.Key("v", "paste image"), Msg: pasteImageMsg{}})
	if s.content.field.Value() != "" {
		actions = append(actions, ui.Action{Key: ui.Key("x", "discard draft"), Msg: discardMsg{}})
	}

	return append(actions, ui.Action{Key: ui.Key("esc", "back", "ctrl+o"), Msg: closeMenuMsg{}})
}

// menuView renders the composer menu as a box of its keys.
func (s Screen) menuView() string {
	actions := s.menuActions()
	width := 0
	for _, action := range actions {
		width = max(width, lipgloss.Width(action.Key.Help().Key))
	}

	var lines []string
	for _, action := range actions {
		k := action.Key.Help().Key
		lines = append(lines, ui.AccentStyle.Render(k)+strings.Repeat(" ", width-lipgloss.Width(k)+2)+action.Key.Help().Desc)
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPrimary).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

// Typing reports whether the composer or the filter takes typed text. The menu, the preview and the discard
// confirmation don't.
func (s Screen) Typing() bool {
	return s.composing && !s.previewing && !s.confirmDiscard && !s.menu || s.content.filter.Typing()
}

// Unsaved reports whether the composer is open.
func (s Screen) Unsaved() bool {
	return s.composing
}

func (s Screen) Status() *ui.Status {
	return s.content.status
}

func (s Screen) textWidth() int {
	if s.width == 0 {
		return 74
	}

	return max(s.width-6, 20)
}

func (s Screen) postButton(post sdk.CommunityPost, indent string) *ui.Button {
	width := s.textWidth() - lipgloss.Width(indent)
	line := FirstLine(post)
	if post.Deleted() {
		line = ui.MutedStyle.Render(line)
	}

	title := indent + ansi.Truncate(styledMeta(post), width, "…") + "\n" +
		indent + ansi.Truncate(line, width, "…")

	return ui.NewButton(title, screen.Send(OpenMsg{Post: post.Descriptor()}))
}

// styledMeta returns author and details of post, with the author in bold.
func styledMeta(post sdk.CommunityPost) string {
	author, details := metaParts(post)
	return ui.BoldStyle.Render(author) + ui.MutedStyle.Render(" · "+details)
}

func metaParts(post sdk.CommunityPost) (string, string) {
	if post.Deleted() {
		return "[deleted]", Ago(post.Instant)
	}

	parts := []string{Ago(post.Instant)}
	if post.Edited {
		parts = append(parts, "edited")
	}

	if previews := post.ReplyPreviews; len(previews) > 0 {
		replies := "replies from " + previews[0].Nickname.Value()
		if len(previews) > 1 {
			replies += fmt.Sprintf(" +%d", len(previews)-1)
		}

		parts = append(parts, replies)
	}

	return post.Owner.Nickname.Value(), strings.Join(parts, " · ")
}

// FirstLine returns the first line of post text for previews. It shows images as [image], removes markdown marks and
// turns shortcodes into emoji.
func FirstLine(post sdk.CommunityPost) string {
	if post.Deleted() {
		return "this post was deleted"
	}

	text := imagePattern.ReplaceAllString(post.Text.Value(), "[image]")
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	line = blockMarkPattern.ReplaceAllString(line, "")
	return ui.Emojize(inlineMarks.Replace(inlineLinkPattern.ReplaceAllString(line, "$1")))
}

// Ago returns short relative time of t, like "5m ago", or its date when older than a day.
func Ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}

	return t.Local().Format("Jan 2")
}

func (s Screen) header() string {
	if s.picking {
		return ui.MutedStyle.Render("links in this post")
	}

	return ""
}

// openedIndex returns list index of the opened post, which follows its parents.
func (s Screen) openedIndex() int {
	return len(s.details.Upstream)
}

// opened renders the opened post in full, as the item after its parents in post mode.
func (s Screen) opened() string {
	post := s.details.Post
	body := ui.MutedStyle.Render("this post was deleted")
	if !post.Deleted() {
		body = s.body(post.Text.Value())
	}

	return ansi.Truncate(styledMeta(post), s.textWidth(), "…") + "\n" + body
}

// body renders post text as markdown wrapped to screen width and draws its images in place.
func (s Screen) body(text string) string {
	var parts []string
	last := 0
	for _, match := range imagePattern.FindAllStringSubmatchIndex(text, -1) {
		if segment := strings.TrimSpace(text[last:match[0]]); segment != "" {
			parts = append(parts, s.markdown.Render(segment, s.textWidth()))
		}

		parts = append(parts, s.picture(text[match[2]:match[3]]))
		last = match[1]
	}

	if segment := strings.TrimSpace(text[last:]); segment != "" {
		parts = append(parts, s.markdown.Render(segment, s.textWidth()))
	}

	return strings.Join(parts, "\n")
}

// renderedKey returns the key of the half-block drawing of url that fits width x rows cells.
func renderedKey(url string, width, rows int) string {
	return fmt.Sprintf("%s@%dx%d", url, width, rows)
}

func (s Screen) picture(url string) string {
	p, ok := s.pictures[url]
	switch {
	case !ok || !p.done:
		return ui.MutedStyle.Render("[loading image]")
	case p.img == nil:
		return ui.MutedStyle.Render("[image unavailable]")
	case p.id != 0:
		return ui.Placeholder(p.id, p.cols, p.rows)
	}

	rows := s.imageRows()
	key := renderedKey(url, s.textWidth(), rows)
	if drawing, ok := s.rendered[key]; ok {
		return drawing
	}

	drawing := ui.RenderImage(p.img, s.textWidth(), rows)
	s.rendered[key] = drawing
	return drawing
}

// composer renders the text field pinned above the list, and the path prompt or the menu below it.
func (s Screen) composer() string {
	// leave room for input border and padding
	s.content.field.Raw().SetWidth(s.textWidth() - 4)
	// one cell narrower than the text field, since a single line input draws an extra cell for the cursor
	s.content.prompt.Raw().SetWidth(s.textWidth() - 5)

	title, placeholder := "new post", "Write a post"
	switch {
	case s.editing != nil:
		title, placeholder = "edit post", "Edit your post"
	case s.mode == modePost:
		author, _ := metaParts(s.replyTo)
		title, placeholder = "reply to "+author, "Write a reply"
	}

	// the length shows past 4000 characters, as on the web
	text := s.draft()
	if n := utf8.RuneCountInString(text); n > 4000 {
		title += fmt.Sprintf(" · %d / %d", n, s.content.field.Raw().CharLimit)
	}

	s.content.field.Raw().Placeholder = placeholder
	field := s.content.field.View()
	if s.previewing {
		title += " · preview"
		field = s.preview(text)
	}

	parts := []string{ui.MutedStyle.Render(title), field}
	switch {
	case s.attaching:
		parts = append(parts, s.content.prompt.View())
		if s.fileInfo != "" {
			parts = append(parts, s.fileInfo)
		}

		if len(s.files) > 0 {
			parts = append(parts, s.filesView())
		}
	case s.menu:
		parts = append(parts, s.menuView())
	}

	if found := s.suggestions(); len(found) > 0 {
		parts = append(parts, s.suggestionsView(found))
	}

	return strings.Join(parts, "\n")
}

// shortcodeQuery returns the name of the shortcode being typed before the cursor, like sm for :sm, or "" when there is
// none. It needs two characters, as one would match too many.
func (s Screen) shortcodeQuery() string {
	field := s.content.field.Raw()
	line := []rune(strings.Split(field.Value(), "\n")[field.Line()])
	match := shortcodeQueryPattern.FindStringSubmatch(string(line[:field.Column()]))
	if match == nil {
		return ""
	}

	return match[1]
}

// suggestions returns shortcodes for the one being typed in the text field, unless esc hid them.
func (s Screen) suggestions() []ui.Shortcode {
	if !s.composing || s.previewing || s.menu || s.confirmDiscard || s.attaching {
		return nil
	}

	query := s.shortcodeQuery()
	if query == "" || query == s.hidden {
		return nil
	}

	return ui.SuggestShortcodes(query, suggestionLimit)
}

// filesView lists path completions with the chosen one marked, up to suggestionLimit around it.
func (s Screen) filesView() string {
	chosen := min(s.suggestion, len(s.files)-1)
	first := max(chosen-suggestionLimit+1, 0)
	last := min(first+suggestionLimit, len(s.files))
	var lines []string
	for i, name := range s.files[first:last] {
		name = ansi.Truncate(name, s.textWidth()-2, "…")
		if first+i == chosen {
			lines = append(lines, ui.AccentStyle.Render("› "+name))
		} else {
			lines = append(lines, ui.MutedStyle.Render("  "+name))
		}
	}

	if more := len(s.files) - last + first; more > 0 {
		lines = append(lines, ui.MutedStyle.Render(fmt.Sprintf("  %d more", more)))
	}

	return strings.Join(lines, "\n")
}

// suggestionsView lists found shortcodes with the chosen one marked. Emoji go last on their lines, so one that the
// terminal draws wider than measured shifts nothing.
func (s Screen) suggestionsView(found []ui.Shortcode) string {
	chosen := min(s.suggestion, len(found)-1)
	lines := make([]string, len(found))
	for i, shortcode := range found {
		name := ":" + shortcode.Name + ":"
		if i == chosen {
			lines[i] = ui.AccentStyle.Render("› "+name) + " " + shortcode.Emoji
		} else {
			lines[i] = ui.MutedStyle.Render("  "+name) + " " + shortcode.Emoji
		}
	}

	return strings.Join(lines, "\n")
}

// previewRows returns the number of lines the preview shows at once, half of the screen like a clipped post.
func (s Screen) previewRows() int {
	return max(s.height/2, 5)
}

// preview renders text as the post will look, from line previewOffset. It cuts a longer draft to previewRows lines and
// adds a position indicator.
func (s Screen) preview(text string) string {
	body := s.body(text)
	lines := strings.Split(body, "\n")
	rows := s.previewRows()
	if len(lines) <= rows {
		return body
	}

	start := min(s.previewOffset, len(lines)-rows)
	indicator := ui.MutedStyle.Render(fmt.Sprintf("lines %d-%d of %d · j/k scroll", start+1, start+rows, len(lines)))
	return strings.Join(lines[start:start+rows], "\n") + "\n" + indicator
}

func (s Screen) View() string {
	if s.user == nil {
		return ui.MutedStyle.Render("log in to see community")
	}

	var top []string
	if header := s.header(); header != "" {
		top = append(top, header)
	}

	if s.mode == modeList {
		if filter := s.content.filter.View(s.textWidth()); filter != "" {
			top = append(top, filter)
		}

		if s.content.filter.Query() != "" && len(s.listed()) == 0 {
			top = append(top, ui.MutedStyle.Render("no posts match"))
		}
	}

	if s.composing {
		top = append(top, s.composer())
	}

	if len(top) == 0 {
		s.content.list.SetHeight(s.height)
		s.content.list.SetTop(0)
		return s.content.list.View()
	}

	header := strings.Join(top, "\n\n")
	if s.height > 0 {
		s.content.list.SetHeight(max(s.height-lipgloss.Height(header)-1, 3))
	}
	// the list starts below the header and the blank line after it
	s.content.list.SetTop(lipgloss.Height(header) + 1)

	return lipgloss.JoinVertical(lipgloss.Left, header, "", s.content.list.View())
}
