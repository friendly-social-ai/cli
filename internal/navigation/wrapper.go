package navigation

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social-ai/cli/internal/router"
	"github.com/friendly-social-ai/cli/internal/ui"
)

// Keys handled by Wrapper itself, shown around the keys of the wrapped model.
var (
	keyMove  = ui.Key("j/k", "move")
	keyQuit  = ui.Key("q", "quit")
	keyHelp  = ui.Key("?", "help")
	keyClose = ui.Key("any key", "close")
)

// Wrapper translates key presses and mouse input into UI messages for the router. While the current screen is typing,
// keys go to it as text, except keys bound to its actions that can't be text.
type Wrapper struct {
	model router.Router

	// typing follows Typing of the router, so that the field gets focus when typing starts and loses it when it stops
	typing bool

	// help shows all keys in place of the screen until the next key press
	help bool
	// pendingG is set after g, which waits for a second g to jump to the first item. Any other key or mouse input
	// cancels it.
	pendingG bool

	width  int
	height int
}

// NewWrapper creates new Wrapper around router.
func NewWrapper(model router.Router) Wrapper {
	return Wrapper{
		model: model,
	}
}

func (w Wrapper) Init() tea.Cmd {
	return w.model.Init()
}

func (w Wrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	w, cmd := w.update(msg)
	return w.follow(cmd)
}

// follow focuses the field of the current screen when it starts typing, and unfocuses it when it stops.
func (w Wrapper) follow(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	typing := w.model.Typing()
	if typing == w.typing {
		return w, cmd
	}

	w.typing = typing
	var focus tea.Msg = ui.UnfocusMsg{}
	if typing {
		focus = ui.FocusMsg{}
	}

	return w, tea.Batch(cmd, func() tea.Msg {
		return focus
	})
}

func (w Wrapper) update(msg tea.Msg) (Wrapper, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height
		msg.Height -= lipgloss.Height(w.footer())

		var cmd tea.Cmd
		w.model, cmd = w.model.Update(msg)
		return w, cmd
	case tea.MouseWheelMsg:
		w.pendingG = false
		// the wheel scrolls the list. It does nothing while typing, since the cursor could follow the scroll off the field.
		if !w.typing && !w.help {
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

		// while typing, the field under the cursor loses focus and the field under the click gets it
		cmds := make([]tea.Cmd, 3)
		if w.typing {
			w.model, cmds[0] = w.model.Update(ui.UnfocusMsg{})
		}

		w.model, cmds[1] = w.model.Update(ui.ClickMsg{X: msg.X, Y: msg.Y})
		if w.typing && w.model.Typing() {
			cmds[2] = func() tea.Msg {
				return ui.FocusMsg{}
			}
		}

		return w, tea.Batch(cmds...)
	case ui.ClickedMsg:
		// a click on the selected item works like enter
		if msg.Again && !w.typing {
			return w, func() tea.Msg {
				return ui.InteractMsg{}
			}
		}

		return w, nil
	case tea.MouseMsg:
		return w, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return w, tea.Quit
		}

		if w.help {
			w.help = false
			return w, nil
		}

		if w.typing {
			if isShortcut(msg, w.keys()) {
				var cmd tea.Cmd
				w.model, cmd = w.model.Update(ui.ActionMsg{Key: msg})
				return w, cmd
			}

			break
		}

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
		case "?":
			w.help = true
			return w, nil
		case "enter", "l", "right":
			return w, func() tea.Msg {
				return ui.InteractMsg{}
			}
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

		// remaining keys are screen actions. Raw keys reach the model only while typing.
		var cmd tea.Cmd
		w.model, cmd = w.model.Update(ui.ActionMsg{Key: msg})
		return w, cmd
	}

	var cmd tea.Cmd
	w.model, cmd = w.model.Update(msg)
	return w, cmd
}

// keys returns key bindings currently offered by the router.
func (w Wrapper) keys() []key.Binding {
	return w.model.Keys()
}

// untypable reports whether k can't be typed as text, like enter, esc or ctrl and alt combinations.
func untypable(k string) bool {
	return len([]rune(k)) > 1
}

// isShortcut reports whether msg can't be text and one of bindings has it, so it works while typing.
func isShortcut(msg tea.KeyPressMsg, bindings []key.Binding) bool {
	if !untypable(msg.String()) {
		return false
	}

	for _, binding := range bindings {
		if binding.Enabled() && key.Matches(msg, binding) {
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
func (w Wrapper) helpView() string {
	everywhere := []key.Binding{keyMove, ui.Key("h/l", "back/open"), ui.Key("gg/G", "first/last item"),
		ui.Key("ctrl+d/u", "half page")}
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

func (w Wrapper) footer() string {
	// tail stays visible when the rest of hints is cut to the width
	var hints, tail string
	switch {
	case w.help:
		hints = renderKeys([]key.Binding{keyClose})
	case w.pendingG:
		// a pending g has no timeout, as in vim, so the footer shows what the next g does
		hints = ui.AccentStyle.Render("g") + ui.MutedStyle.Render(" pending") + keySeparator +
			renderKeys([]key.Binding{ui.Key("g", "first item")})
	case w.typing:
		// only keys that can't be text work while typing
		var bindings []key.Binding
		for _, binding := range w.keys() {
			if untypable(binding.Help().Key) {
				bindings = append(bindings, binding)
			}
		}

		hints = renderKeys(bindings)
	default:
		hints = renderKeys(w.keys())
		tail = renderKeys([]key.Binding{keyHelp})
	}

	room := w.width - 2
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
		Render(hints)
}

func (w Wrapper) View() tea.View {
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
