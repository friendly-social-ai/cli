package community

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social/cli/internal/ui"
	sdk "github.com/friendly-social/golang-sdk"
)

// imagePattern matches markdown image and captures its URL.
var imagePattern = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)\)`)

// items builds interactive elements of the current mode.
func (s Screen) items() []tea.Model {
	if s.mode == modeList {
		s.content.field.Raw().Placeholder = "Write a post"
		items := []tea.Model{
			s.content.field,
			ui.NewButton("Post", send(submitMsg{})),
			ui.NewButton("Refresh", send(refreshMsg{})),
		}

		for _, post := range s.posts {
			items = append(items, s.postButton(post, ""))
		}

		if s.next != nil {
			items = append(items, ui.NewButton("Load more", send(moreMsg{})))
		}

		return append(items, ui.NewButton("Back", send(backMsg{})))
	}

	post := s.details.Post
	items := []tea.Model{s.content.field}

	switch {
	case s.editing:
		s.content.field.Raw().Placeholder = "Edit your post"
		items = append(items,
			ui.NewButton("Save", send(submitMsg{})),
			ui.NewButton("Cancel", send(cancelEditMsg{})))
	default:
		s.content.field.Raw().Placeholder = "Write a reply"
		items = append(items, ui.NewButton("Reply", send(submitMsg{})))

		if s.owns(post) && !post.Deleted() {
			deleteTitle := "Delete"
			if s.confirmDelete {
				deleteTitle = ui.DangerStyle.Render("Confirm delete")
			}

			items = append(items,
				ui.NewButton("Edit", send(editMsg{})),
				ui.NewButton(deleteTitle, send(deleteMsg{})))
		}
	}

	if n := len(s.details.Upstream); n > 0 {
		parent := s.details.Upstream[n-1]
		items = append(items, ui.NewButton("Open parent", send(openMsg{post: parent.Descriptor()})))
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

	if s.repliesNext != nil {
		items = append(items, ui.NewButton("Load more replies", send(moreMsg{})))
	}

	return append(items, ui.NewButton("Back", send(backMsg{})))
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

	return ui.NewButton(title, send(openMsg{post: post.Descriptor()}))
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
