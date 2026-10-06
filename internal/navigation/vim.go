package navigation

import (
	"slices"
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
	keyMove  = ui.Key("j/k", "move")
	keyQuit  = ui.Key("q", "quit")
	keyDone  = ui.Key("esc", "done")
	keyHelp  = ui.Key("?", "help")
	keyClose = ui.Key("any key", "close")
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

	// help shows all keys in place of the screen until the next key press
	help bool

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
	case tea.MouseWheelMsg:
		// the wheel moves like j and k. It does nothing while typing, since moving would leave the field.
		if w.mode == VimModeNormal && !w.help {
			switch msg.Button {
			case tea.MouseWheelDown:
				return w, func() tea.Msg {
					return ui.MoveMsg{Direction: ui.DirectionDown}
				}
			case tea.MouseWheelUp:
				return w, func() tea.Msg {
					return ui.MoveMsg{Direction: ui.DirectionUp}
				}
			}
		}

		return w, nil
	case tea.MouseMsg:
		return w, nil
	case tea.KeyPressMsg:
		if w.help {
			w.help = false
			if msg.String() == "ctrl+c" {
				return w, tea.Quit
			}

			return w, nil
		}

		switch w.mode {
		case VimModeNormal:
			switch msg.String() {
			case "q", "ctrl+c":
				return w, tea.Quit
			case "?":
				w.help = true
				return w, nil
			case "i", "enter":
				// i and enter start typing only where the model offers it, so the footer matches. Elsewhere enter interacts.
				if typable(w.keys()) {
					w.mode = VimModeInsert
					return w, func() tea.Msg {
						return ui.FocusMsg{}
					}
				}

				if msg.String() == "enter" {
					return w, func() tea.Msg {
						return ui.InteractMsg{}
					}
				}

				return w, nil
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

// typable reports whether bindings offer typing with i.
func typable(bindings []key.Binding) bool {
	for _, binding := range bindings {
		if binding.Enabled() && slices.Contains(binding.Keys(), "i") {
			return true
		}
	}

	return false
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

	return strings.Join(hints, keySeparator)
}

// keySeparator goes between key hints.
var keySeparator = ui.MutedStyle.Render(" · ")

// helpView lists keys of the current screen, then the ones that work on every screen.
func (w VimWrapper) helpView() string {
	everywhere := []key.Binding{keyMove, ui.Key("ctrl+d/u", "scroll a long item"), ui.Key("esc", "stop typing")}
	if w.model.OnTab() {
		everywhere = append(everywhere, ui.Key("1-4", "switch tabs"))
	}

	everywhere = append(everywhere, keyHelp, keyQuit)
	return helpSection("this screen", w.keys()) + "\n\n" + helpSection("everywhere", everywhere)
}

// helpSection renders title over bindings, one per line with aligned descriptions.
func helpSection(title string, bindings []key.Binding) string {
	width := 0
	for _, binding := range bindings {
		width = max(width, lipgloss.Width(binding.Help().Key))
	}

	lines := []string{ui.BoldStyle.Render(title)}
	for _, binding := range bindings {
		if binding.Enabled() {
			k := binding.Help().Key
			lines = append(lines, "  "+ui.AccentStyle.Render(k)+strings.Repeat(" ", width-lipgloss.Width(k)+2)+binding.Help().Desc)
		}
	}

	return strings.Join(lines, "\n")
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
	// tail stays visible when the rest of hints is cut to the width
	var hints, tail string
	switch {
	case w.help:
		hints = renderKeys([]key.Binding{keyClose})
	case w.mode == VimModeNormal:
		var bindings []key.Binding
		for _, binding := range w.keys() {
			// where enter starts typing, its bindings work only while typing
			if !typable(w.keys()) || !slices.Contains(binding.Keys(), "enter") {
				bindings = append(bindings, binding)
			}
		}

		hints = renderKeys(bindings)
		tail = renderKeys([]key.Binding{keyHelp})
	case w.mode == VimModeInsert:
		bindings := []key.Binding{keyDone}
		for _, binding := range w.keys() {
			if k, ok := shortcut(binding); ok {
				bindings = append(bindings, ui.Key(k, binding.Help().Desc))
			}
		}

		hints = renderKeys(bindings)
	}

	room := w.width - lipgloss.Width(badge) - 4
	if tail != "" {
		room -= lipgloss.Width(tail) + lipgloss.Width(keySeparator)
	}

	hints = ansi.Truncate(hints, max(room, 0), "…")
	if hints != "" && tail != "" {
		hints += keySeparator
	}
	hints += tail

	return lipgloss.NewStyle().
		Width(w.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ui.ColorBorder).
		Render(badge + "  " + hints)
}

func (w VimWrapper) View() tea.View {
	footer := w.footer()

	content := w.model.View()
	if w.help {
		content = w.model.Header() + "\n" + lipgloss.NewStyle().Padding(1, 1).Render(w.helpView())
	}

	content = ui.Clip(content, w.width, w.height-lipgloss.Height(footer))
	view := tea.NewView(content + "\n" + footer)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}
