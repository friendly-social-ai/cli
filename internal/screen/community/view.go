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

var (
	dimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	boldStyle = lipgloss.NewStyle().Bold(true)
)

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
				deleteTitle = "Confirm delete"
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
				indent = "  "
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
	width := s.textWidth() - len(indent)
	title := indent + ansi.Truncate(meta(post), width, "…") + "\n" +
		indent + ansi.Truncate(firstLine(post), width, "…")

	return ui.NewButton(title, send(openMsg{post: post.Descriptor()}))
}

func meta(post sdk.CommunityPost) string {
	if post.Deleted() {
		return "[deleted] · " + when(post.Instant)
	}

	parts := []string{post.Owner.Nickname.Value(), when(post.Instant)}
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

	return strings.Join(parts, " · ")
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
		return "community"
	}

	var lines []string
	for _, post := range s.details.Upstream {
		lines = append(lines, dimStyle.Render(ansi.Truncate("↑ "+meta(post)+": "+firstLine(post), s.textWidth(), "…")))
	}

	post := s.details.Post
	body := "this post was deleted"
	if !post.Deleted() {
		body = s.body(post.Text.Value())
	}

	lines = append(lines,
		boldStyle.Render(ansi.Truncate(meta(post), s.textWidth(), "…")),
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
		return dimStyle.Render("[loading image]")
	case p.img == nil:
		return dimStyle.Render("[image unavailable]")
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
	s.content.field.Raw().SetWidth(s.textWidth())

	top := s.header()
	if status := s.content.status.View(); status != "" {
		top = lipgloss.JoinVertical(lipgloss.Left, top, status)
	}

	if s.height > 0 {
		s.content.list.SetHeight(max(s.height-lipgloss.Height(top)-1, 3))
	}

	return lipgloss.JoinVertical(lipgloss.Left, top, "", s.content.list.View())
}
