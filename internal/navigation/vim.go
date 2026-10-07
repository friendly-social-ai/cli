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
	// pendingG is set after g, which waits for a second g to jump to the first item. Any other key or mouse input
	// cancels it.
	pendingG bool

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
		w.pendingG = false
		// the wheel scrolls the list. It does nothing while typing, since the cursor could follow the scroll off the field.
		if w.mode == VimModeNormal && !w.help {
			switch msg.Button {
			case tea.MouseWheelDown:
				return w, func() tea.Msg {
					return ui.WheelMsg{Direction: ui.DirectionDown}
				}
			case tea.MouseWheelUp:
				return w, func() tea.Msg {
					return ui.WheelMsg{Direction: ui.DirectionUp}
				}
			}
		}

		return w, nil
	case tea.MouseClickMsg:
		w.pendingG = false
		if msg.Button != tea.MouseLeft || msg.Y >= w.height-lipgloss.Height(w.footer()) {
			return w, nil
		}

		if w.help {
			w.help = false
			return w, nil
		}

		// a click stops typing first, so that the click reaches the list and not the field
		var unfocus tea.Cmd
		if w.mode == VimModeInsert {
			w.mode = VimModeNormal
			w.model, unfocus = w.model.Update(ui.UnfocusMsg{})
		}

		var cmd tea.Cmd
		w.model, cmd = w.model.Update(ui.ClickMsg{X: msg.X, Y: msg.Y})
		return w, tea.Batch(unfocus, cmd)
	case ui.ClickedMsg:
		// a click on a field starts typing, and a click on the selected item works like enter
		if typable(w.keys()) || msg.Again {
			return w.enter()
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
			if w.pendingG {
				w.pendingG = false
				if msg.String() == "g" {
					return w, func() tea.Msg {
						return ui.JumpMsg{Direction: ui.DirectionUp}
					}
				}
			}

			switch msg.String() {
			case "q":
				// q is likely a slip while the screen has typed text. ctrl+c still quits.
				if w.model.Unsaved() {
					return w, nil
				}

				return w, tea.Quit
			case "ctrl+c":
				return w, tea.Quit
			case "?":
				w.help = true
				return w, nil
			case "i":
				if typable(w.keys()) {
					return w.enter()
				}

				return w, nil
			case "enter":
				return w.enter()
			case "left":
				// left works like h. Both reach the screen as an action below, and screens bind h to go back.
				msg = tea.KeyPressMsg{Code: 'h', Text: "h"}
			case "j", "down":
				return w, func() tea.Msg {
					return ui.MoveMsg{Direction: ui.DirectionDown}
				}
			case "k", "up":
				return w, func() tea.Msg {
					return ui.MoveMsg{Direction: ui.DirectionUp}
				}
			case "l", "right":
				// l opens the selected item like enter, but never starts typing
				if typable(w.keys()) {
					return w, nil
				}

				return w, func() tea.Msg {
					return ui.InteractMsg{}
				}
			case "g":
				w.pendingG = true
				return w, nil
			case "G":
				return w, func() tea.Msg {
					return ui.JumpMsg{Direction: ui.DirectionDown}
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

// enter starts typing where the model offers i, so the footer matches. Elsewhere it sends InteractMsg to the
// selected item.
func (w VimWrapper) enter() (tea.Model, tea.Cmd) {
	if typable(w.keys()) {
		w.mode = VimModeInsert
		return w, func() tea.Msg {
			return ui.FocusMsg{}
		}
	}

	return w, func() tea.Msg {
		return ui.InteractMsg{}
	}
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
	everywhere := []key.Binding{keyMove, ui.Key("h/l", "back/open"), ui.Key("gg/G", "first/last item"),
		ui.Key("ctrl+d/u", "half page"), ui.Key("esc", "stop typing")}
	if w.model.OnTab() {
		everywhere = append(everywhere, ui.Key("1-4", "switch tabs"))
	}

	everywhere = append(everywhere, keyHelp)
	if !w.model.Unsaved() {
		everywhere = append(everywhere, keyQuit)
	}

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
	case w.pendingG:
		// a pending g has no timeout, as in vim, so the footer shows what the next g does
		hints = ui.AccentStyle.Render("g") + ui.MutedStyle.Render(" pending") + keySeparator +
			renderKeys([]key.Binding{ui.Key("g", "first item")})
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
