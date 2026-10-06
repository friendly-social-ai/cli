package router

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/friendly-social/cli/internal/screen"
	"github.com/friendly-social/cli/internal/ui"
)

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
	cmds := make([]tea.Cmd, 0, len(r.screens))
	for _, s := range r.screens {
		cmds = append(cmds, s.Init())
	}

	return tea.Batch(cmds...)
}

func (r Router) target(target screen.Type, msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	r.screens[target], cmd = r.screens[target].Update(msg)
	return r, cmd
}

func (r Router) broadcast(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0, len(r.screens))

	for i := range r.screens {
		r.screens[i], cmd = r.screens[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	return r, tea.Batch(cmds...)
}

func (r Router) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height

		msg.Height -= lipgloss.Height(r.header())
		return r.broadcast(msg)
	case screen.ChangeMsg:
		r.current = msg.NewType
		return r, nil
	case TargetMsg:
		return r.target(msg.Type, msg.Inner)
	case BroadcastMsg:
		return r.broadcast(msg.Inner)
	}

	return r.target(r.current, msg)
}

func (r Router) header() string {
	return lipgloss.NewStyle().
		Width(r.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ui.ColorBorder).
		Render(ui.AccentStyle.Render("friendly") + ui.MutedStyle.Render(" · "+string(r.current)))
}

func (r Router) View() string {
	header := r.header()

	// clip screens taller than the window instead of pushing the header out
	content := ui.Clip(r.screens[r.current].View(), r.width, r.height-lipgloss.Height(header))
	return header + "\n" + content
}
