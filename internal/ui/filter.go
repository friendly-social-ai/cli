package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// Filter narrows a list to items whose text contains its query, ignoring case. It receives typed keys from Start until
// the next UnfocusMsg, and keeps the query after that.
type Filter struct {
	field  *Field
	typing bool
}

// NewFilter returns empty Filter.
func NewFilter() *Filter {
	input := textinput.New()
	input.Prompt = "/ "
	input.Placeholder = "filter"
	return &Filter{field: NewField(input)}
}

// Start lets the next messages type into the query.
func (f *Filter) Start() {
	f.typing = true
}

// Typing reports whether messages go to the query.
func (f *Filter) Typing() bool {
	return f.typing
}

// Query returns the text items must contain.
func (f *Filter) Query() string {
	return f.field.Value()
}

// Clear empties the query.
func (f *Filter) Clear() {
	f.field.Raw().SetValue("")
}

// Match reports whether text contains the query.
func (f *Filter) Match(text string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(f.Query()))
}

// Update passes msg to the query while typing. It reports whether the query changed, and stops typing on UnfocusMsg.
func (f *Filter) Update(msg tea.Msg) (bool, tea.Cmd) {
	if _, ok := msg.(UnfocusMsg); ok {
		f.typing = false
	}

	before := f.Query()
	_, cmd := f.field.Update(msg)
	return f.Query() != before, cmd
}

// View renders the query field while typing, and the query alone after that. It is empty without a query.
func (f *Filter) View(width int) string {
	switch {
	case f.typing:
		f.field.Raw().SetWidth(width - 6)
		return f.field.View()
	case f.Query() != "":
		return MutedStyle.Render("/ " + f.Query())
	}

	return ""
}
