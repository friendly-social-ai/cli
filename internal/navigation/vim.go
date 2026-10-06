package navigation

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
				// only where the model offers typing, so that footer and behaviour agree
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
			}

			// the rest are screen actions, raw keys go to the model only in insert mode for typing
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

			// shortcuts work while typing, printable keys always go to the text
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

// keys returns key bindings currently offered by the wrapped model.
func (w VimWrapper) keys() []key.Binding {
	if model, ok := w.model.(interface{ Keys() []key.Binding }); ok {
		return model.Keys()
	}

	return nil
}

// shortcut returns ctrl or alt key of binding, usable while typing since it can't be text.
func shortcut(binding key.Binding) (string, bool) {
	for _, k := range binding.Keys() {
		if strings.HasPrefix(k, "ctrl+") || strings.HasPrefix(k, "alt+") {
			return k, binding.Enabled()
		}
	}

	return "", false
}

// isShortcut reports whether msg is a shortcut of one of bindings.
func isShortcut(msg tea.KeyMsg, bindings []key.Binding) bool {
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

func (w VimWrapper) View() string {
	footer := w.footer()

	content := ui.Clip(w.model.View(), w.width, w.height-lipgloss.Height(footer))
	return content + "\n" + footer
}
