package router

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/ui"
)

// tabs are the screens of a logged in user, shown in the header and switched with their keys.
var tabs = []struct {
	key    string
	screen screen.Type
	title  string
}{
	{"1", screen.TypeCommunity, "Community"},
	{"2", screen.TypeActivity, "Activity"},
	{"3", screen.TypePeople, "People"},
	{"4", screen.TypeProfile, "Profile"},
}

// Router orchestrates multiple screens.
type Router struct {
	current screen.Type
	screens map[screen.Type]screen.Model

	width  int
	height int
}

// NewRouter creates new Router based on provided screens.
func NewRouter(models []screen.Model) Router {
	screens := make(map[screen.Type]screen.Model)
	for _, m := range models {
		screens[m.ID()] = m
	}

	return Router{
		current: models[0].ID(),
		screens: screens,
	}
}

func (r Router) Init() tea.Cmd {
	cmds := []tea.Cmd{minute()}
	for _, s := range r.screens {
		cmds = append(cmds, s.Init())
	}

	return tea.Batch(cmds...)
}

// minute returns command delivering screen.MinuteMsg in a minute.
func minute() tea.Cmd {
	return tea.Tick(time.Minute, func(time.Time) tea.Msg {
		return screen.MinuteMsg{}
	})
}

func (r Router) target(target screen.Type, msg tea.Msg) (Router, tea.Cmd) {
	var cmd tea.Cmd
	r.screens[target], cmd = r.screens[target].Update(msg)
	return r, cmd
}

func (r Router) broadcast(msg tea.Msg) (Router, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0, len(r.screens))

	for i := range r.screens {
		r.screens[i], cmd = r.screens[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	return r, tea.Batch(cmds...)
}

func (r Router) Update(msg tea.Msg) (Router, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height

		msg.Height -= lipgloss.Height(r.Header())
		return r.broadcast(msg)
	case screen.ChangeMsg:
		r.current = msg.NewType
		return r, nil
	case TargetMsg:
		return r.target(msg.Type, msg.Inner)
	case BroadcastMsg:
		return r.broadcast(msg.Inner)
	case screen.MinuteMsg:
		r, cmd := r.broadcast(msg)
		return r, tea.Batch(cmd, minute())
	case ui.ClickMsg:
		header := lipgloss.Height(r.Header())
		if msg.Y >= header {
			msg.Y -= header
			return r.target(r.current, msg)
		}

		if tab, ok := r.tabAt(msg.X); ok && msg.Y == 0 {
			r.current = tab
		}

		return r, nil
	case ui.ActionMsg:
		if r.OnTab() {
			for _, tab := range tabs {
				if msg.Key.String() == tab.key {
					r.current = tab.screen
					return r, nil
				}
			}
		}
	}

	return r.target(r.current, msg)
}

// Keys returns key bindings of the current screen.
func (r Router) Keys() []key.Binding {
	return r.screens[r.current].Keys()
}

// Unsaved reports whether the current screen holds typed text that quitting would lose. A screen with an Unsaved
// method can hold some.
func (r Router) Unsaved() bool {
	holder, ok := r.screens[r.current].(interface{ Unsaved() bool })
	return ok && holder.Unsaved()
}

// OnTab reports whether the current screen is one of tabs, which digit keys switch.
func (r Router) OnTab() bool {
	for _, tab := range tabs {
		if tab.screen == r.current {
			return true
		}
	}

	return false
}

// brand starts the title line, and tabSeparator goes between its parts.
const (
	brand        = "friendly"
	tabSeparator = "  "
)

// title names the current screen, or lists tabs with the current one highlighted.
func (r Router) title() string {
	head := ui.AccentStyle.Render(brand)
	if !r.OnTab() {
		return head + ui.MutedStyle.Render(" · "+string(r.current))
	}

	return strings.Join(append([]string{head}, r.tabLabels()...), tabSeparator)
}

// tabAt returns the tab drawn at column x of the title line.
func (r Router) tabAt(x int) (screen.Type, bool) {
	if !r.OnTab() {
		return "", false
	}

	// one column of header padding comes before the brand
	start := 1 + len(brand)
	for i, label := range r.tabLabels() {
		start += len(tabSeparator)
		end := start + lipgloss.Width(label)
		if x >= start && x < end {
			return tabs[i].screen, true
		}

		start = end
	}

	return "", false
}

// tabLabels renders a label for each of tabs with the current one highlighted. A screen with a Badge method shows
// its badge next to its tab.
func (r Router) tabLabels() []string {
	var labels []string
	for _, tab := range tabs {
		label := ui.MutedStyle.Render("[" + tab.key + "] " + tab.title)
		if tab.screen == r.current {
			label = ui.MutedStyle.Render("["+tab.key+"] ") + ui.AccentStyle.Render(tab.title)
		}

		if badged, ok := r.screens[tab.screen].(interface{ Badge() string }); ok && badged.Badge() != "" {
			label += " " + ui.AccentStyle.Render(badged.Badge())
		}

		labels = append(labels, label)
	}

	return labels
}

// Header renders the title line over screens, with the status of the current screen.
func (r Router) Header() string {
	// the title is cut to one line. The status goes to the right end, cut to the room the title leaves.
	inner := r.width - 2
	title := ansi.Truncate(r.title(), max(inner, 0), "…")
	if status := r.screens[r.current].Status(); status != "" && inner-lipgloss.Width(title)-2 > 0 {
		status = ansi.Truncate(status, inner-lipgloss.Width(title)-2, "…")
		title += strings.Repeat(" ", inner-lipgloss.Width(title)-lipgloss.Width(status)) + status
	}

	return lipgloss.NewStyle().
		Width(r.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ui.ColorBorder).
		Render(title)
}

func (r Router) View() string {
	header := r.Header()

	// clip screens taller than the window instead of pushing the header out
	content := ui.Clip(r.screens[r.current].View(), r.width, r.height-lipgloss.Height(header))
	return header + "\n" + content
}
