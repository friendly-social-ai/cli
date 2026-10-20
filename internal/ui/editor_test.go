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
	view := ansi.Strip(e.View())
	// the box adds a border row above and below the text, and the status line goes under it
	if got, want := strings.Count(view, "\n")+1, 4+3; got != want {
		t.Errorf("View() has %d lines, want %d:\n%s", got, want, view)
	}

	if strings.Contains(view, "1") || !strings.Contains(view, "5") {
		t.Errorf("View() = %q, want 2 to 5", view)
	}
}
