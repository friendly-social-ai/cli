package navigation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/friendly-social-ai/cli/internal/keys"
	"github.com/friendly-social-ai/cli/internal/logging"
	"github.com/friendly-social-ai/cli/internal/router"
	"github.com/friendly-social-ai/cli/internal/ui"
)

// minWidth and minHeight are the smallest terminal the layout fits. A smaller one shows only a note about its size.
const minWidth, minHeight = 40, 12

// Wrapper translates key presses and mouse input into UI messages for the router. While the current screen is typing,
// keys go to it as text, except keys bound to its actions that can't be text.
type Wrapper struct {
	model router.Router

	// typing follows Typing of the router, so that the field gets focus when typing starts and loses it when it stops
	typing bool

	// help shows the keys of the current screen and the app in a panel over the screen. A key other than ? and esc
	// closes it and works as usual. helpOffset is the first help line shown when the lines don't fit.
	help       bool
	helpOffset int
	// pending holds the keys of a sequence so far, joined by spaces, while it waits for the next key. A key that doesn't
	// continue it, or mouse input, cancels it.
	pending string

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
	defer logging.Recover()
	w.trace(msg)
	w, cmd := w.update(msg)
	return w.follow(cmd)
}

// trace logs msg at the debug level by its type, and a message for one screen by the type it carries. A key logs its
// name, except one typed as text while the screen is typing, so drafts, emails and codes stay out of the log.
func (w Wrapper) trace(msg tea.Msg) {
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	switch msg := msg.(type) {
	case spinner.TickMsg:
		// the spinner ticks many times a second while loading
	case tea.KeyPressMsg:
		k := msg.String()
		if w.typing && (!untypable(k) || k == "space") {
			k = "typed"
		}

		slog.Debug("key", "key", k, "typing", w.typing)
	case tea.PasteMsg:
		slog.Debug("paste", "length", len(msg.Content))
	case router.TargetMsg:
		slog.Debug("msg", "type", fmt.Sprintf("%T", msg.Inner), "screen", msg.Type)
	case router.BroadcastMsg:
		slog.Debug("msg", "type", fmt.Sprintf("%T", msg.Inner), "screen", "all")
	default:
		// the cursor of a text field blinks twice a second
		if name := fmt.Sprintf("%T", msg); !strings.HasPrefix(name, "cursor.") {
			slog.Debug("msg", "type", name)
		}
	}
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
	// a terminal too small for the layout hides the screen, but input would still change it. Only ctrl+c goes through.
	if w.tooSmall() {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			if msg.String() != "ctrl+c" {
				return w, nil
			}
		case tea.MouseMsg:
			return w, nil
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height
		msg.Height -= lipgloss.Height(w.footer())

		var cmd tea.Cmd
		w.model, cmd = w.model.Update(msg)
		return w, cmd
	case tea.MouseWheelMsg:
		w.pending = ""
		if w.help {
			w.scrollHelp(msg.Button == tea.MouseWheelUp)
			return w, nil
		}

		// the wheel scrolls the list. It does nothing while typing, since the cursor could follow the scroll off the field.
		if !w.typing {
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
		w.pending = ""
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

		k := msg.String()
		if w.help {
			switch {
			case keys.Navigation.Help.Matches(k) || keys.Common.Cancel.Matches(k):
				w.help = false
				return w, nil
			case keys.Navigation.Down.Matches(k) || keys.Navigation.Up.Matches(k):
				if w.helpOverflows() {
					w.scrollHelp(keys.Navigation.Up.Matches(k))
					return w, nil
				}
			}

			w.help = false
		}

		if w.typing {
			if isShortcut(msg, w.keys()) {
				var cmd tea.Cmd
				w.model, cmd = w.model.Update(ui.ActionMsg{Keys: k})
				return w, cmd
			}

			break
		}

		if w.pending != "" {
			k, w.pending = w.pending+" "+k, ""
		}

		if len(w.completions(k)) > 0 {
			w.pending = k
			return w, nil
		}

		switch {
		case keys.Navigation.Quit.Matches(k):
			// quitting is likely a slip while the screen has typed text. ctrl+c still quits.
			if w.model.Unsaved() {
				return w, nil
			}

			return w, tea.Quit
		case keys.Navigation.Help.Matches(k):
			w.help, w.helpOffset = true, 0
			return w, nil
		case keys.Navigation.Open.Matches(k):
			return w, func() tea.Msg {
				return ui.InteractMsg{}
			}
		case keys.Navigation.Down.Matches(k):
			return w, func() tea.Msg {
				return ui.MoveMsg{Direction: ui.DirectionDown}
			}
		case keys.Navigation.Up.Matches(k):
			return w, func() tea.Msg {
				return ui.MoveMsg{Direction: ui.DirectionUp}
			}
		case keys.Navigation.First.Matches(k):
			return w, func() tea.Msg {
				return ui.JumpMsg{Direction: ui.DirectionUp}
			}
		case keys.Navigation.Last.Matches(k):
			return w, func() tea.Msg {
				return ui.JumpMsg{Direction: ui.DirectionDown}
			}
		case keys.Navigation.HalfPageDown.Matches(k):
			return w, func() tea.Msg {
				return ui.ScrollMsg{Direction: ui.DirectionDown}
			}
		case keys.Navigation.HalfPageUp.Matches(k):
			return w, func() tea.Msg {
				return ui.ScrollMsg{Direction: ui.DirectionUp}
			}
		}

		// remaining keys are screen actions, like back. Raw keys reach the model only while typing.
		var cmd tea.Cmd
		w.model, cmd = w.model.Update(ui.ActionMsg{Keys: k})
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

// navigationKeys returns bindings of the keys the wrapper handles, described for the footer.
func (w Wrapper) navigationKeys() []key.Binding {
	n := keys.Navigation
	bindings := []key.Binding{keys.Bind("quit", n.Quit), keys.Bind("help", n.Help), keys.Bind("open", n.Open),
		keys.Bind("down", n.Down), keys.Bind("up", n.Up), keys.Bind("first item", n.First),
		keys.Bind("last item", n.Last), keys.Bind("half page down", n.HalfPageDown),
		keys.Bind("half page up", n.HalfPageUp)}
	if w.model.OnTab() {
		for i, tab := range n.Tabs {
			bindings = append(bindings, keys.Bind(fmt.Sprintf("tab %d", i+1), tab))
		}
	}

	return bindings
}

// completions returns hints for the keys that complete a sequence starting with prefix, or nil when no sequence
// does.
func (w Wrapper) completions(prefix string) []key.Binding {
	var hints []key.Binding
	for _, binding := range append(w.navigationKeys(), w.keys()...) {
		for _, k := range binding.Keys() {
			if rest, ok := strings.CutPrefix(k, prefix+" "); ok {
				hints = append(hints, keys.Hint(binding.Help().Desc, rest))
			}
		}
	}

	return hints
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

// helpKeys returns the sections of the help panel: keys of the current screen, moves, and keys of the app.
func (w Wrapper) helpKeys() []helpSection {
	var app []key.Binding
	if w.model.OnTab() {
		tabs := keys.Navigation.Tabs
		app = append(app, keys.Hint("switch tabs, press again for the first view", tabs[0].Key()+"-"+tabs[len(tabs)-1].Key()))
	}

	if !w.model.Unsaved() {
		app = append(app, keys.Bind("quit", keys.Navigation.Quit))
	}

	// navigation handles the moves itself, so these bindings only label keys in the panel
	return []helpSection{
		{"Screen", w.keys()},
		{"Move", []key.Binding{
			keys.Hint("down / up", keys.Pair(keys.Navigation.Down, keys.Navigation.Up, " / ")...),
			keys.Hint("first / last item", keys.Pair(keys.Navigation.First, keys.Navigation.Last, " / ")...),
			keys.Hint("half page down / up", keys.Pair(keys.Navigation.HalfPageDown, keys.Navigation.HalfPageUp, " / ")...),
		}},
		{"App", app},
	}
}

// helpSection is a titled group of keys in the help panel.
type helpSection struct {
	title    string
	bindings []key.Binding
}

// arrows replaces the names of arrow keys with their symbols in the help panel.
var arrows = strings.NewReplacer("left", "←", "right", "→", "up", "↑", "down", "↓")

// helpKey renders the keys of binding, its main key first and the alternatives after it.
func helpKey(binding key.Binding) string {
	rendered := ui.AccentStyle.Render(arrows.Replace(binding.Help().Key))
	for _, k := range binding.Keys()[1:] {
		rendered += "  " + ui.MutedStyle.Render(arrows.Replace(k))
	}

	return rendered
}

// helpLines renders the sections of the help panel, one key per line with descriptions aligned across sections.
func (w Wrapper) helpLines() []string {
	sections := w.helpKeys()
	width := 0
	for _, section := range sections {
		for _, binding := range section.bindings {
			width = max(width, lipgloss.Width(helpKey(binding)))
		}
	}

	var lines []string
	for _, section := range sections {
		if len(section.bindings) == 0 {
			continue
		}

		if lines != nil {
			lines = append(lines, "")
		}

		lines = append(lines, ui.BoldStyle.Render(section.title))
		for _, binding := range section.bindings {
			if binding.Enabled() {
				k := helpKey(binding)
				lines = append(lines, "  "+k+strings.Repeat(" ", width-lipgloss.Width(k)+2)+binding.Help().Desc)
			}
		}
	}

	return lines
}

// helpRows returns the number of help lines the panel shows at once. Its border, padding and the lines about
// scrolling take 6 rows of the room between the header and the footer.
func (w Wrapper) helpRows() int {
	return max(w.height-lipgloss.Height(w.model.Header())-lipgloss.Height(w.footer())-6, 1)
}

// helpOverflows reports whether the help lines don't fit the panel, so the down and up keys scroll them.
func (w Wrapper) helpOverflows() bool {
	return len(w.helpLines()) > w.helpRows()
}

// scrollHelp scrolls the help lines one line up or down, stopping at either end.
func (w *Wrapper) scrollHelp(up bool) {
	step := 1
	if up {
		step = -1
	}

	w.helpOffset = max(min(w.helpOffset+step, len(w.helpLines())-w.helpRows()), 0)
}

// helpView renders the help panel centered in a width x height area, titled with the name of the current screen.
func (w Wrapper) helpView(width, height int) string {
	lines := w.helpLines()
	// the border and the padding take 6 columns
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, max(width-6, 1), "…")
	}

	if rows := w.helpRows(); len(lines) > rows {
		start := min(w.helpOffset, len(lines)-rows)
		indicator := fmt.Sprintf("lines %d-%d of %d · %s scroll", start+1, start+rows, len(lines),
			keys.Label(keys.Navigation.Down, keys.Navigation.Up, "/"))
		lines = append(lines[start:start+rows], "", ui.MutedStyle.Render(indicator))
	}

	body := lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(lines, "\n"))
	border := lipgloss.NewStyle().Foreground(ui.ColorBorder)
	// the top border is drawn here to hold the title, and the style draws the other three sides
	inner := lipgloss.Width(body)
	title := " " + ansi.Truncate(w.model.Name(), max(inner-3, 0), "…") + " "
	top := border.Render("╭─") + ui.BoldStyle.Render(title) +
		border.Render(strings.Repeat("─", max(inner-1-lipgloss.Width(title), 0))+"╮")
	rest := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), false, true, true, true).
		BorderForeground(ui.ColorBorder).
		Render(body)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, top+"\n"+rest)
}

func (w Wrapper) footer() string {
	// tail stays visible when the rest of hints is cut to the width
	var hints, tail string
	switch {
	case w.help:
		hints = renderKeys([]key.Binding{keys.Hint("close", keys.Navigation.Help.Key()+" / "+keys.Common.Cancel.Key())})
	case w.pending != "":
		// a pending sequence has no timeout, as in vim, so the footer shows what completes it
		hints = ui.AccentStyle.Render(w.pending) + ui.MutedStyle.Render(" pending") + keySeparator +
			renderKeys(w.completions(w.pending))
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
		tail = renderKeys([]key.Binding{keys.Bind("help", keys.Navigation.Help)})
	}

	// the position in the list goes to the right end, like the status in the header
	var position string
	if i, n := w.model.Position(); n > 0 && !w.help {
		position = ui.MutedStyle.Render(fmt.Sprintf("%d/%d", i, n))
	}

	room := w.width - 2
	if tail != "" {
		room -= lipgloss.Width(tail) + lipgloss.Width(keySeparator)
	}

	if position != "" {
		room -= lipgloss.Width(position) + 2
	}

	hints = ansi.Truncate(hints, max(room, 0), "…")
	if hints != "" && tail != "" {
		hints += keySeparator
	}
	hints += tail
	if position != "" {
		hints += strings.Repeat(" ", max(w.width-2-lipgloss.Width(hints)-lipgloss.Width(position), 2)) + position
	}

	return lipgloss.NewStyle().
		Width(w.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ui.ColorBorder).
		Render(hints)
}

func (w Wrapper) View() tea.View {
	defer logging.Recover()
	view := tea.NewView(w.content())
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

// content renders the screen with the footer, or the note about the size when the terminal is too small.
func (w Wrapper) content() string {
	if w.tooSmall() {
		size := fmt.Sprintf("%dx%d, needs %dx%d", w.width, w.height, minWidth, minHeight)
		return lipgloss.Place(w.width, w.height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center, ui.BoldStyle.Render("terminal too small"), ui.MutedStyle.Render(size)))
	}

	footer := w.footer()
	content := w.model.View()
	if w.help {
		header := w.model.Header()
		content = header + "\n" + w.helpView(w.width, w.height-lipgloss.Height(header)-lipgloss.Height(footer))
	}

	return ui.Clip(ui.SplitConjuncts(content), w.width, w.height-lipgloss.Height(footer)) + "\n" + footer
}

// tooSmall reports whether the terminal is smaller than the layout fits.
func (w Wrapper) tooSmall() bool {
	return w.width > 0 && (w.width < minWidth || w.height < minHeight)
}
