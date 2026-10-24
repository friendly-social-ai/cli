package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

var esc = tea.KeyPressMsg{Code: tea.KeyEscape}

// newTestEditor returns a focused Editor in insert mode, indenting by 4 and holding up to limit characters.
func newTestEditor(limit int) *Editor {
	e := NewEditor(4, limit)
	e.SetWidth(20)
	e.Update(FocusMsg{})
	e.StartInsert()
	return e
}

// typeText presses a key for each character of text, enter for each line break.
func typeText(e *Editor, text string) {
	for _, r := range text {
		if r == '\n' {
			e.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			continue
		}

		e.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestEditorRunsVimCommands(t *testing.T) {
	e := newTestEditor(100)
	typeText(e, "hello big world")
	e.Update(esc)
	typeText(e, "bbdw")
	if got, want := e.Value(), "hello world"; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}
}

func TestEditorUndoesInsertAsOneStep(t *testing.T) {
	e := newTestEditor(100)
	typeText(e, "one")
	e.Update(esc)
	typeText(e, "otwo")
	e.Update(esc)
	typeText(e, "u")
	if got, want := e.Value(), "one"; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}
}

func TestEditorRefusesTextPastLimit(t *testing.T) {
	e := newTestEditor(5)
	e.SetWidth(40)
	typeText(e, "abcdef")
	if got, want := e.Value(), "abcde"; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}

	if view := ansi.Strip(e.View()); !strings.Contains(view, "draft too long, 6 / 5") {
		t.Errorf("View() = %q, want the limit error", view)
	}

	if err := e.Replace(0, 0, "x"); err == nil {
		t.Error("Replace past the limit succeeded")
	}
}

func TestEditorEscIsIdleOnlyInNormalMode(t *testing.T) {
	e := newTestEditor(100)
	if e.Idle() {
		t.Error("Idle() in insert mode")
	}

	e.Update(esc)
	if !e.Idle() {
		t.Error("not Idle() after esc")
	}

	typeText(e, "d")
	if e.Idle() {
		t.Error("Idle() while d waits for a motion")
	}
}

func TestEditorTabIndentsWithSpaces(t *testing.T) {
	e := newTestEditor(100)
	typeText(e, "a")
	e.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if got, want := e.Value(), "a   "; got != want {
		t.Errorf("Value() after tab = %q, want %q", got, want)
	}

	e.SetValue("        a")
	e.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if got, want := e.Value(), "    a"; got != want {
		t.Errorf("Value() after shift+tab = %q, want %q", got, want)
	}
}

func TestEditorReplaceMovesCursorAfterIt(t *testing.T) {
	e := newTestEditor(100)
	typeText(e, "abcd")
	if err := e.Replace(1, 3, "xyz"); err != nil {
		t.Fatal(err)
	}

	if got, want := e.Value(), "axyzd"; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}

	if got, want := e.Cursor(), 5; got != want {
		t.Errorf("Cursor() = %d, want %d", got, want)
	}
}

func TestEditorScrollsToCursor(t *testing.T) {
	e := newTestEditor(100)
	typeText(e, "one\ntwo\nthree\nfour\nfive")
	view := ansi.Strip(e.View())
	if strings.Contains(view, "two") || !strings.Contains(view, "five") {
		t.Errorf("View() = %q, want three to five", view)
	}

	e.Update(esc)
	typeText(e, "gg")
	view = ansi.Strip(e.View())
	if !strings.Contains(view, "one") || strings.Contains(view, "four") {
		t.Errorf("View() after gg = %q, want one to three", view)
	}
}

func TestEditorSetValueKeepsInsertMode(t *testing.T) {
	e := newTestEditor(100)
	e.SetValue("draft")
	typeText(e, "!")
	if got, want := e.Value(), "draft!"; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}
}

func TestEditorStartInsertWhileInsertingKeepsCursor(t *testing.T) {
	e := newTestEditor(100)
	e.SetValue("abc")
	e.StartInsert()
	typeText(e, "d")
	if got, want := e.Value(), "abcd"; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}
}

func TestEditorKeepsIndentOnNewLine(t *testing.T) {
	e := newTestEditor(100)
	typeText(e, "  a\nb")
	if got, want := e.Value(), "  a\n  b"; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}
}

func TestEditorPastesWithTabsAsIndent(t *testing.T) {
	e := newTestEditor(100)
	e.Update(tea.PasteMsg{Content: "x\ty\r\nz"})
	if got, want := e.Value(), "x    y\nz"; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}
}

func TestEditorMarksWrappedRowsOnly(t *testing.T) {
	e := newTestEditor(100)
	typeText(e, "one two three four five six\nseven")
	lines := strings.Split(ansi.Strip(e.View()), "\n")
	// the box draws a border row first, and the text wraps at 19 columns
	if !strings.HasSuffix(lines[1], "↩ │") {
		t.Errorf("wrapped row = %q, want a marker", lines[1])
	}

	for _, line := range lines[2:4] {
		if strings.Contains(line, "↩") {
			t.Errorf("row %q ends a line, want no marker", line)
		}
	}
}

func TestEditorGrowsUpToMaxRows(t *testing.T) {
	e := newTestEditor(100)
	e.SetMaxRows(4)
	typeText(e, "1\n2\n3\n4\n5")
	lines := strings.Split(ansi.Strip(e.View()), "\n")
	// the box adds a border row above and below the text
	if got, want := len(lines), 4+2; got != want {
		t.Fatalf("View() has %d lines, want %d:\n%s", got, want, strings.Join(lines, "\n"))
	}

	text := strings.Join(lines[1:len(lines)-1], "\n")
	if strings.Contains(text, "1") || !strings.Contains(text, "5") {
		t.Errorf("text rows = %q, want 2 to 5", text)
	}

	if !strings.Contains(lines[0], "↑ 1 more") {
		t.Errorf("top border = %q, want the hidden row counted", lines[0])
	}
}

func TestEditorDrawsEveryRegionAndCursor(t *testing.T) {
	e := newTestEditor(100)
	typeText(e, "foo x foo y foo")
	e.Update(esc)
	typeText(e, "0")
	for range 3 {
		e.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	}

	if got := strings.Count(e.View(), selectionStyle.Render("f")); got != 3 {
		t.Errorf("View() selects %d regions, want 3", got)
	}

	typeText(e, "cbar")
	if got, want := e.Value(), "bar x bar y bar"; got != want {
		t.Fatalf("Value() = %q, want %q", got, want)
	}

	// the terminal draws the main cursor, and the other two draw as cells
	view, cursor := TakeCursor(e.View())
	if got := strings.Count(view, cursorStyle.Render(" ")); got != 2 || cursor == nil {
		t.Errorf("View() draws %d cursors and the terminal cursor %v, want 2 and one", got, cursor)
	}
}

func TestEditorShowsControlCharactersAsCaret(t *testing.T) {
	e := newTestEditor(100)
	e.Update(tea.PasteMsg{Content: "a\x01b\x7f"})
	view, cursor := TakeCursor(e.View())
	if view := ansi.Strip(view); !strings.Contains(view, "a^Ab^?") {
		t.Errorf("View() = %q, want a^Ab^?", view)
	}

	// the cursor sits after the six cells, past the border and padding columns
	if cursor == nil || cursor.X != 8 {
		t.Errorf("cursor = %+v, want X 8", cursor)
	}
}

func TestEditorPlacesTerminalCursorByMode(t *testing.T) {
	e := newTestEditor(100)
	typeText(e, "ab\ncd")
	// the box draws a border row and a border and padding column before the text
	_, cursor := TakeCursor(e.View())
	if cursor == nil || cursor.X != 4 || cursor.Y != 2 || cursor.Shape != tea.CursorBar {
		t.Errorf("cursor in insert mode = %+v, want a bar at 4, 2", cursor)
	}

	e.Update(esc)
	_, cursor = TakeCursor(e.View())
	if cursor == nil || cursor.X != 3 || cursor.Y != 2 || cursor.Shape != tea.CursorBlock {
		t.Errorf("cursor in normal mode = %+v, want a block at 3, 2", cursor)
	}

	// the command line starts in the bottom border, after its corner, a line and a space
	typeText(e, ":s")
	_, cursor = TakeCursor(e.View())
	if cursor == nil || cursor.X != 5 || cursor.Y != 4 || cursor.Shape != tea.CursorBar {
		t.Errorf("cursor on the command line = %+v, want a bar at 5, 4", cursor)
	}

	e.Update(UnfocusMsg{})
	if view, cursor := TakeCursor(e.View()); cursor != nil || strings.Contains(view, markStart) {
		t.Errorf("unfocused cursor = %+v, want none", cursor)
	}
}

func TestEditorMovesCursorToCommandLineOnEmptyDraft(t *testing.T) {
	e := newTestEditor(100)
	e.Placeholder = "Write a post"
	e.Update(esc)
	typeText(e, ":")
	if _, cursor := TakeCursor(e.View()); cursor == nil || cursor.Y != 4 {
		t.Errorf("cursor = %+v, want it on the command line", cursor)
	}
}

// command types line on the command line from normal mode and returns the message its enter sends, or nil.
func command(e *Editor, line string) tea.Msg {
	e.Update(esc)
	typeText(e, line)
	_, cmd := e.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		return nil
	}

	return cmd()
}

func TestEditorWritesAndQuits(t *testing.T) {
	for _, tc := range []struct {
		line string
		want tea.Msg
	}{
		{":w", WriteMsg{}},
		{":wq", WriteMsg{}},
		{":x", WriteMsg{}},
		{":q", QuitMsg{}},
		{":q!", QuitMsg{Discard: true}},
	} {
		e := newTestEditor(100)
		typeText(e, "draft")
		if got := command(e, tc.line); got != tc.want {
			t.Errorf("%s sends %#v, want %#v", tc.line, got, tc.want)
		}
	}
}

func TestEditorWritesAndQuitsWithZ(t *testing.T) {
	for _, tc := range []struct {
		keys string
		want tea.Msg
	}{{"ZZ", WriteMsg{}}, {"ZQ", QuitMsg{Discard: true}}} {
		e := newTestEditor(100)
		typeText(e, "draft")
		e.Update(esc)
		e.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
		_, cmd := e.Update(tea.KeyPressMsg{Code: rune(tc.keys[1]), Text: tc.keys[1:]})
		if cmd == nil || cmd() != tc.want {
			t.Errorf("%s sends %v, want %#v", tc.keys, cmd, tc.want)
		}
	}
}

func TestEditorRefusesOtherCommands(t *testing.T) {
	for _, tc := range []struct{ draft, line, want string }{
		{"", ":w", "the draft is empty"},
		{"draft", ":e notes.md", "not available here: :e notes.md"},
	} {
		e := newTestEditor(100)
		// wide enough for the error in the bottom border
		e.SetWidth(60)
		typeText(e, tc.draft)
		if got := command(e, tc.line); got != nil {
			t.Errorf("%s sends %#v, want nothing", tc.line, got)
		}

		if view := ansi.Strip(e.View()); !strings.Contains(view, tc.want) {
			t.Errorf("View() after %s = %q, want %q", tc.line, view, tc.want)
		}
	}
}
