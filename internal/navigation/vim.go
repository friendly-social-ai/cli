package navigation

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social/cli/internal/router"
	"github.com/friendly-social/cli/internal/ui"
)

// Keys handled by VimWrapper itself, shown around the keys of the wrapped model.
var (
	keyMove = ui.Key("j/k", "move")
	keyQuit = ui.Key("q", "quit")
	keyDone = ui.Key("esc", "done")
)

// VimMode represents possible modes for Vim motions.
type VimMode string

const (
	VimModeNormal VimMode = "NORMAL"
	VimModeInsert VimMode = "INSERT"
)

// VimWrapper translates key presses into UI messages for the router with Vim-style modes and motions.
type VimWrapper struct {
	mode  VimMode
	model router.Router

	width  int
	height int
}

// NewVimWrapper creates new VimWrapper around router.
func NewVimWrapper(model router.Router) VimWrapper {
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
	case ui.InsertMsg:
		w.mode = VimModeInsert
		return w, func() tea.Msg {
			return ui.FocusMsg{}
		}
	case ui.NormalMsg:
		w.mode = VimModeNormal
		return w, func() tea.Msg {
			return ui.UnfocusMsg{}
		}
	case tea.KeyPressMsg:
		switch w.mode {
		case VimModeNormal:
			switch msg.String() {
			case "q", "ctrl+c":
				return w, tea.Quit
			case "i":
				// enter insert mode only where the model offers typing, so the footer matches the behaviour
				if !key.Matches(msg, w.keys()...) {
					return w, nil
				}

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
			case "ctrl+d":
				return w, func() tea.Msg {
					return ui.ScrollMsg{Direction: ui.DirectionDown}
				}
			case "ctrl+u":
				return w, func() tea.Msg {
					return ui.ScrollMsg{Direction: ui.DirectionUp}
				}
			}

			// remaining keys are screen actions. Raw keys reach the model only in insert mode, for typing.
			var cmd tea.Cmd
			w.model, cmd = w.model.Update(ui.ActionMsg{Key: msg})
			return w, cmd
		case VimModeInsert:
			switch msg.String() {
			case "esc", "ctrl+c":
				w.mode = VimModeNormal
				return w, func() tea.Msg {
					return ui.UnfocusMsg{}
				}
			}

			if isShortcut(msg, w.keys()) {
				var cmd tea.Cmd
				w.model, cmd = w.model.Update(ui.ActionMsg{Key: msg})
				return w, cmd
			}
		}
	}

	var cmd tea.Cmd
	w.model, cmd = w.model.Update(msg)
	return w, cmd
}

// keys returns key bindings currently offered by the router.
func (w VimWrapper) keys() []key.Binding {
	return w.model.Keys()
}

// shortcut returns the key of binding that works while typing, one that can't be text, like enter or ctrl and alt
// combinations. Esc doesn't count, since it always stops typing.
func shortcut(binding key.Binding) (string, bool) {
	for _, k := range binding.Keys() {
		if len([]rune(k)) > 1 && k != "esc" {
			return k, binding.Enabled()
		}
	}

	return "", false
}

// isShortcut reports whether msg is a shortcut of one of bindings.
func isShortcut(msg tea.KeyPressMsg, bindings []key.Binding) bool {
	for _, binding := range bindings {
		if k, ok := shortcut(binding); ok && msg.String() == k {
			return true
		}
	}

	return false
}

func renderKeys(bindings []key.Binding) string {
	var hints []string
	for _, binding := range bindings {
		if binding.Enabled() {
			hints = append(hints, binding.Help().Key+" "+ui.MutedStyle.Render(binding.Help().Desc))
		}
	}

	return strings.Join(hints, ui.MutedStyle.Render(" · "))
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
	var hints string
	switch w.mode {
	case VimModeNormal:
		hints = renderKeys(append(append([]key.Binding{keyMove}, w.keys()...), keyQuit))
	case VimModeInsert:
		bindings := []key.Binding{keyDone}
		for _, binding := range w.keys() {
			if k, ok := shortcut(binding); ok {
				bindings = append(bindings, ui.Key(k, binding.Help().Desc))
			}
		}

		hints = renderKeys(bindings)
	}

	hints = ansi.Truncate(hints, max(w.width-lipgloss.Width(badge)-4, 0), "…")

	return lipgloss.NewStyle().
		Width(w.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ui.ColorBorder).
		Render(badge + "  " + hints)
}

func (w VimWrapper) View() tea.View {
	footer := w.footer()

	content := ui.Clip(w.model.View(), w.width, w.height-lipgloss.Height(footer))
	view := tea.NewView(content + "\n" + footer)
	view.AltScreen = true
	return view
}
