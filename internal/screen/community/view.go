package community

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
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
)

// items builds elements of the current mode: the opened post in post mode, then the text field followed by posts.
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

	if s.mode == modePost {
		items = append(items, ui.NewLabel(s.opened()))
	}

	items = append(items, s.content.field)
	if s.attaching {
		items = append(items, s.content.prompt)
	}

	if s.mode == modeList {
		s.content.field.Raw().Placeholder = "Write a post"
		for _, post := range s.posts {
			items = append(items, s.postButton(post, ""))
		}

		return items
	}

	s.content.field.Raw().Placeholder = "Write a reply"
	if s.editing {
		s.content.field.Raw().Placeholder = "Edit your post"
	}

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

// actions builds keys available in the current state. The cursor on the text field offers writing, on a post opening.
func (s Screen) actions() []ui.Action {
	if s.attaching {
		return []ui.Action{{Key: ui.Key("enter", "attach"), Msg: attachDoneMsg{}}}
	}

	if s.picking {
		return []ui.Action{
			{Key: ui.Key("enter", "open")},
			{Key: ui.Key("esc", "cancel"), Msg: cancelPickMsg{}},
		}
	}

	var actions []ui.Action
	if s.content.list.Scrollable() {
		actions = append(actions, ui.Action{Key: ui.Key("ctrl+d/u", "scroll")})
	}

	switch cursor := s.content.list.Cursor(); {
	case cursor == s.fieldIndex():
		actions = append(actions, ui.Action{Key: ui.Key("i", "write")})
		if s.content.field.Value() != "" {
			actions = append(actions, ui.Action{Key: ui.Key("p", s.submitLabel(), "alt+enter"), Msg: submitMsg{}})
		}

		actions = append(actions, ui.Action{Key: ui.Key("a", "attach"), Msg: attachMsg{}})
	case cursor > s.fieldIndex():
		actions = append(actions, ui.Action{Key: ui.Key("enter", "open")})
	case cursor < s.fieldIndex() && !s.details.Post.Deleted():
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
	}

	refresh := ui.Action{Key: ui.Key("r", "refresh"), Msg: refreshMsg{}}
	back := ui.Action{Key: ui.Key("esc", "back"), Msg: backMsg{}}

	if s.mode == modeList {
		return append(actions, refresh, back)
	}

	switch {
	case s.editing:
		return append(actions, ui.Action{Key: ui.Key("esc", "cancel"), Msg: cancelEditMsg{}})
	case s.confirmDelete:
		return append(actions,
			ui.Action{Key: ui.Key("d", "confirm delete"), Msg: deleteMsg{}},
			ui.Action{Key: ui.Key("esc", "cancel"), Msg: cancelDeleteMsg{}})
	}

	if post := s.details.Post; s.owns(post) && !post.Deleted() {
		actions = append(actions,
			ui.Action{Key: ui.Key("e", "edit"), Msg: editMsg{}},
			ui.Action{Key: ui.Key("d", "delete"), Msg: deleteMsg{}})
	}

	if n := len(s.details.Upstream); n > 0 {
		actions = append(actions, ui.Action{Key: ui.Key("u", "parent"), Msg: OpenMsg{Post: s.details.Upstream[n-1].Descriptor()}})
	}

	return append(actions, refresh, back)
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
	case s.editing:
		return "save"
	}

	return "reply"
}

func (s Screen) Keys() []key.Binding {
	return ui.Keys(s.actions())
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

func meta(post sdk.CommunityPost) string {
	author, details := metaParts(post)
	return author + " · " + details
}

// styledMeta is meta with emphasized author.
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

	if len(post.ReplyPreviews) > 0 {
		names := make([]string, len(post.ReplyPreviews))
		for i, user := range post.ReplyPreviews {
			names[i] = user.Nickname.Value()
		}

		parts = append(parts, "replies from "+strings.Join(names, ", "))
	}

	return post.Owner.Nickname.Value(), strings.Join(parts, " · ")
}

// FirstLine returns the first line of post text for previews. It shows images as [image] and removes markdown marks.
func FirstLine(post sdk.CommunityPost) string {
	if post.Deleted() {
		return "this post was deleted"
	}

	text := imagePattern.ReplaceAllString(post.Text.Value(), "[image]")
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	line = blockMarkPattern.ReplaceAllString(line, "")
	return inlineMarks.Replace(inlineLinkPattern.ReplaceAllString(line, "$1"))
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
	if s.mode == modeList {
		return ""
	}

	if s.picking {
		return ui.MutedStyle.Render("links in this post")
	}

	var lines []string
	for _, post := range s.details.Upstream {
		lines = append(lines, ui.MutedStyle.Render(ansi.Truncate("↑ "+meta(post)+": "+FirstLine(post), s.textWidth(), "…")))
	}

	return strings.Join(lines, "\n")
}

// opened renders the opened post in full, as the first item of post mode.
func (s Screen) opened() string {
	post := s.details.Post
	body := ui.MutedStyle.Render("this post was deleted")
	if !post.Deleted() {
		body = s.body(post.Text.Value())
	}

	return ansi.Truncate(styledMeta(post), s.textWidth(), "…") + "\n" + body
}

// fieldIndex returns list index of the text field, which follows the opened post in post mode.
func (s Screen) fieldIndex() int {
	if s.mode == modePost {
		return 1
	}

	return 0
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
	key := fmt.Sprintf("%s@%dx%d", url, s.textWidth(), rows)
	if drawing, ok := s.rendered[key]; ok {
		return drawing
	}

	drawing := ui.RenderImage(p.img, s.textWidth(), rows)
	s.rendered[key] = drawing
	return drawing
}

func (s Screen) View() string {
	// leave room for input border and padding
	s.content.field.Raw().SetWidth(s.textWidth() - 4)
	// one cell narrower than the text field, since a single line input draws an extra cell for the cursor
	s.content.prompt.Raw().SetWidth(s.textWidth() - 5)

	var top []string
	for _, part := range []string{s.header(), s.content.status.View()} {
		if part != "" {
			top = append(top, part)
		}
	}

	if len(top) == 0 {
		s.content.list.SetHeight(s.height)
		return s.content.list.View()
	}

	header := strings.Join(top, "\n")
	if s.height > 0 {
		s.content.list.SetHeight(max(s.height-lipgloss.Height(header)-1, 3))
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, "", s.content.list.View())
}
