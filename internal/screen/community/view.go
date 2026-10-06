package community

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
)

// imagePattern matches markdown image and captures its URL.
var imagePattern = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)\)`)

// items builds elements of the current mode: the text field followed by posts.
func (s Screen) items() []tea.Model {
	items := []tea.Model{s.content.field}

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

// actions builds keys available in the current state. Cursor on the text field offers writing, on a post opening.
func (s Screen) actions() []ui.Action {
	var actions []ui.Action
	if s.content.list.Cursor() == 0 {
		actions = append(actions, ui.Action{Key: ui.Key("i", "write")})
		if s.content.field.Value() != "" {
			actions = append(actions, ui.Action{Key: ui.Key("p", s.submitLabel()), Msg: submitMsg{}})
		}
	} else {
		actions = append(actions, ui.Action{Key: ui.Key("enter", "open")})
	}

	refresh := ui.Action{Key: ui.Key("r", "refresh"), Msg: refreshMsg{}}
	more := ui.Action{Key: ui.Key("m", "more"), Msg: moreMsg{}}
	back := ui.Action{Key: ui.Key("esc", "back"), Msg: backMsg{}}

	if s.mode == modeList {
		actions = append(actions, refresh)
		if s.next != nil {
			actions = append(actions, more)
		}

		return append(actions, back)
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
		actions = append(actions, ui.Action{Key: ui.Key("u", "parent"), Msg: openMsg{post: s.details.Upstream[n-1].Descriptor()}})
	}

	actions = append(actions, refresh)
	if s.repliesNext != nil {
		actions = append(actions, more)
	}

	return append(actions, back)
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
	line := firstLine(post)
	if post.Deleted() {
		line = ui.MutedStyle.Render(line)
	}

	title := indent + ansi.Truncate(styledMeta(post), width, "…") + "\n" +
		indent + ansi.Truncate(line, width, "…")

	return ui.NewButton(title, screen.Send(openMsg{post: post.Descriptor()}))
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
		return "[deleted]", when(post.Instant)
	}

	parts := []string{when(post.Instant)}
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

func firstLine(post sdk.CommunityPost) string {
	if post.Deleted() {
		return "this post was deleted"
	}

	text := imagePattern.ReplaceAllString(post.Text.Value(), "[image]")
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return line
}

func when(t time.Time) string {
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

	var lines []string
	for _, post := range s.details.Upstream {
		lines = append(lines, ui.MutedStyle.Render(ansi.Truncate("↑ "+meta(post)+": "+firstLine(post), s.textWidth(), "…")))
	}

	post := s.details.Post
	body := ui.MutedStyle.Render("this post was deleted")
	if !post.Deleted() {
		body = s.body(post.Text.Value())
	}

	lines = append(lines,
		ansi.Truncate(styledMeta(post), s.textWidth(), "…"),
		body)

	// leave room for the reply field and a few list items below
	return lipgloss.NewStyle().
		MaxHeight(max(s.height-10, 3)).
		Render(strings.Join(lines, "\n"))
}

// body renders post text wrapped to screen width with markdown images drawn in place.
func (s Screen) body(text string) string {
	style := lipgloss.NewStyle().Width(s.textWidth())

	var parts []string
	last := 0
	for _, match := range imagePattern.FindAllStringSubmatchIndex(text, -1) {
		if segment := strings.TrimSpace(text[last:match[0]]); segment != "" {
			parts = append(parts, style.Render(segment))
		}

		parts = append(parts, s.picture(text[match[2]:match[3]]))
		last = match[1]
	}

	if segment := strings.TrimSpace(text[last:]); segment != "" {
		parts = append(parts, style.Render(segment))
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
