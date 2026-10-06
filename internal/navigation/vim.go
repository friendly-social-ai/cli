package navigation

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social/cli/internal/ui"
)

// vimHints lists keys available in each mode.
var vimHints = map[VimMode]string{
	VimModeNormal: "j/k move · enter select · i type · q quit",
	VimModeInsert: "esc stop typing",
}

// VimMode represents possible modes for Vim motions.
type VimMode string

const (
	VimModeNormal VimMode = "NORMAL"
	VimModeInsert VimMode = "INSERT"
)

// VimWrapper translates raw tea.KeyMsgs to UI messages using Vim motions driven logic.
type VimWrapper struct {
	mode  VimMode
	model tea.Model

	width  int
	height int
}

// NewVimWrapper creates new VimWrapper based on provided model.
func NewVimWrapper(model tea.Model) VimWrapper {
	return VimWrapper{
		model: model,
		mode:  VimModeNormal,
	}
}

func (w VimWrapper) Init() tea.Cmd {
	return w.model.Init()
}

func (w VimWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height
		msg.Height -= lipgloss.Height(w.footer())

		var cmd tea.Cmd
		w.model, cmd = w.model.Update(msg)
		return w, cmd
	case tea.KeyMsg:
		switch w.mode {
		case VimModeNormal:
			switch msg.String() {
			case "q", "ctrl+c":
				return w, tea.Quit
			case "i":
				w.mode = VimModeInsert
				return w, func() tea.Msg {
					return ui.FocusMsg{}
				}
			case "h", "left":
				return w, func() tea.Msg {
					return ui.MoveMsg{Direction: ui.DirectionLeft}
				}
			case "j", "down":
				return w, func() tea.Msg {
					return ui.MoveMsg{Direction: ui.DirectionDown}
				}
			case "k", "up":
				return w, func() tea.Msg {
					return ui.MoveMsg{Direction: ui.DirectionUp}
				}
			case "l", "right":
				return w, func() tea.Msg {
					return ui.MoveMsg{Direction: ui.DirectionRight}
				}
			case "enter":
				return w, func() tea.Msg {
					return ui.InteractMsg{}
				}
			}
		case VimModeInsert:
			switch msg.String() {
			case "esc", "ctrl+c":
				w.mode = VimModeNormal
				return w, func() tea.Msg {
					return ui.UnfocusMsg{}
				}
			}
		}
	}

	var cmd tea.Cmd
	w.model, cmd = w.model.Update(msg)
	return w, cmd
}

func (w VimWrapper) footer() string {
	color := ui.ColorPrimary
	if w.mode == VimModeInsert {
		color = ui.ColorSuccess
	}

	badge := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1).
		Foreground(ui.ColorOnAccent).
		Background(color).
		Render(string(w.mode))
	hints := ansi.Truncate(vimHints[w.mode], max(w.width-lipgloss.Width(badge)-4, 0), "…")

	return lipgloss.NewStyle().
		Width(w.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ui.ColorBorder).
		Render(badge + "  " + ui.MutedStyle.Render(hints))
}

func (w VimWrapper) View() string {
	footer := w.footer()

	content := lipgloss.NewStyle().
		Width(w.width).
		Height(w.height - lipgloss.Height(footer)).
		Render(w.model.View())

	return lipgloss.JoinVertical(lipgloss.Top, content, footer)
}
