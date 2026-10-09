package ui

import (
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

// newTestArea returns a focused TextArea holding text, with the cursor at its end.
func newTestArea(text string) *TextArea {
	input := textarea.New()
	input.SetWidth(40)
	area := NewTextArea(input)
	area.Update(FocusMsg{})
	area.Raw().SetValue(text)
	return area
}

func press(area *TextArea, code rune, mod tea.KeyMod) {
	area.Update(tea.KeyPressMsg{Code: code, Mod: mod})
}

// selectDown selects from the start of the text down count lines.
func selectDown(area *TextArea, count int) {
	press(area, tea.KeyHome, tea.ModCtrl)
	for range count {
		press(area, tea.KeyDown, tea.ModShift)
	}
}

func TestIndentWithoutSelectionAddsSpacesToNextLevel(t *testing.T) {
	area := newTestArea("ab")
	area.Indent()
	if got, want := area.Value(), "ab  "; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}
}

func TestIndentShiftsSelectedLinesAndKeepsSelection(t *testing.T) {
	area := newTestArea("a\nb\nc")
	selectDown(area, 2)
	area.Indent()
	area.Indent()

	// the selection ends at the start of the last line, which stays as it is
	if got, want := area.Value(), "        a\n        b\nc"; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}

	start, end, ok := area.Raw().Selection()
	wantStart, wantEnd := textarea.Position{Row: 0, Col: 0}, textarea.Position{Row: 2, Col: 0}
	if !ok || start != wantStart || end != wantEnd {
		t.Errorf("Selection() = %v, %v, %v, want %v, %v, true", start, end, ok, wantStart, wantEnd)
	}
}

func TestOutdentRemovesUpToOneLevel(t *testing.T) {
	area := newTestArea("      a\n  b\nc")
	selectDown(area, 2)
	press(area, tea.KeyRight, tea.ModShift)
	area.Outdent()
	if got, want := area.Value(), "  a\nb\nc"; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}
}

func TestOutdentWithoutSelectionMovesCursorWithText(t *testing.T) {
	area := newTestArea("      ab")
	area.Outdent()
	if got, want := area.Value(), "  ab"; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}

	if got, want := area.Raw().Column(), 4; got != want {
		t.Errorf("Column() = %d, want %d", got, want)
	}
}

func TestIndentPastLimitChangesNothing(t *testing.T) {
	input := textarea.New()
	input.CharLimit = 6
	area := NewTextArea(input)
	area.Update(FocusMsg{})
	area.Raw().SetValue("a\nb")
	selectDown(area, 1)
	press(area, tea.KeyRight, tea.ModShift)
	area.Indent()
	if got, want := area.Value(), "a\nb"; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}
}

func TestNewlineKeepsIndentation(t *testing.T) {
	area := newTestArea("    if x {")
	press(area, tea.KeyEnter, 0)
	if got, want := area.Value(), "    if x {\n    "; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}
}

func TestNewlineOverSelectedLinesKeepsIndentation(t *testing.T) {
	area := newTestArea("    a\nb\nc")
	press(area, tea.KeyHome, tea.ModCtrl)
	press(area, tea.KeyEnd, 0)
	press(area, tea.KeyDown, tea.ModShift)
	press(area, tea.KeyDown, tea.ModShift)
	press(area, tea.KeyEnter, 0)
	if got, want := area.Value(), "    a\n    "; got != want {
		t.Errorf("Value() = %q, want %q", got, want)
	}
}
