package ui

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/y3owk1n/fude/core/kernel"
	"github.com/y3owk1n/fude/vim"
)

// minRows is the fewest rows of text Editor shows.
const minRows = 3

// origin is the kernel origin of every edit, since one person types in an Editor.
const origin kernel.Origin = "me"

// Editor is a multi-line text field with Vim's keys. A kernel buffer holds the text and its undo history, and a vim
// machine turns keys into edits. It wraps at words and scrolls to keep the cursor in view.
type Editor struct {
	buf    *kernel.Buffer
	vim    *vim.Machine
	cursor int
	marks  map[rune]kernel.MarkID
	// before holds the cursor from before each undo step by its node. pending is the cursor when the step being made
	// began, depth counts open groups, and head is the newest node that before has a cursor for.
	before  map[kernel.NodeID]int
	pending int
	depth   int
	head    kernel.NodeID
	// top is the first row shown, counted over the rows of the whole text
	top int
	// maxRows is the most rows of text that show before the text scrolls
	maxRows int
	// width is the columns of text, indent the columns of one level of indentation, and limit the most characters the
	// text may have
	width, indent, limit int
	focused              bool
	// err is why the last key failed, shown until the next key
	err string
	// Placeholder shows while the text is empty.
	Placeholder string
	// Title shows in the top border, and Note on the right of the bottom border.
	Title, Note string
}

// NewEditor creates an empty Editor that indents by indent spaces and holds up to limit characters.
func NewEditor(indent, limit int) *Editor {
	e := &Editor{indent: indent, limit: limit, width: 40, maxRows: minRows}
	e.reset("")
	e.vim = vim.New(host{e: e})
	e.vim.ExpandTab(true)
	// shift+tab outdents in insert mode, as tab indents
	e.vim.Map(vim.Insert, "<S-Tab>", "<C-d>")
	return e
}

func (e *Editor) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case FocusMsg:
		e.focused = true
	case UnfocusMsg:
		e.focused = false
	case tea.PasteMsg:
		if e.focused {
			e.settle(e.vim.Paste(e.untab(msg.Content)))
		}
	case tea.KeyPressMsg:
		if e.focused {
			e.settle(e.feed(keys(msg)))
		}
	}

	return e, nil
}

// feed feeds ks to the machine, stopping at the first that fails.
func (e *Editor) feed(ks []vim.Key) error {
	for _, k := range ks {
		if err := e.vim.Feed(k); err != nil {
			return err
		}
	}

	return nil
}

// settle keeps err for the status line until the next key, and scrolls to the cursor.
func (e *Editor) settle(err error) {
	e.err = ""
	if err != nil {
		e.err = err.Error()
	}

	e.scroll()
}

// namedKeys holds the keys without text by their code, in Vim notation.
var namedKeys = map[rune]vim.Key{
	tea.KeyEnter: "<CR>", tea.KeyEscape: "<Esc>", tea.KeyBackspace: "<BS>", tea.KeyTab: "\t", tea.KeySpace: " ",
	tea.KeyLeft: "<Left>", tea.KeyRight: "<Right>", tea.KeyUp: "<Up>", tea.KeyDown: "<Down>",
	tea.KeyHome: "<Home>", tea.KeyEnd: "<End>", tea.KeyDelete: "<Del>", tea.KeyInsert: "<Insert>",
	tea.KeyPgUp: "<PageUp>", tea.KeyPgDown: "<PageDown>",
}

// keys returns msg in Vim notation, or nothing for a key Vim has no use for. An alt chord is <Esc> and the key, as a
// terminal without the kitty protocol sends it.
func keys(msg tea.KeyPressMsg) []vim.Key {
	mod := msg.Mod &^ tea.ModShift
	switch {
	case msg.Code == tea.KeyTab && msg.Mod&tea.ModShift != 0:
		return []vim.Key{"<S-Tab>"}
	case mod == tea.ModAlt:
		msg.Mod &^= tea.ModAlt
		if k := keys(msg); k != nil {
			return append([]vim.Key{"<Esc>"}, k...)
		}

		return nil
	case mod == 0 && msg.Text != "":
		var ks []vim.Key
		for _, r := range msg.Text {
			ks = append(ks, vim.Key(r))
		}

		return ks
	case mod == tea.ModCtrl && msg.Code >= 'a' && msg.Code <= 'z':
		return []vim.Key{vim.Key("<C-" + string(msg.Code) + ">")}
	case mod != 0:
		return nil
	}

	if k, ok := namedKeys[msg.Code]; ok {
		return []vim.Key{k}
	}

	return nil
}

// untab turns each tab of text into a level of indentation and each line break into \n.
func (e *Editor) untab(text string) string {
	return strings.NewReplacer("\r\n", "\n", "\r", "\n", "\t", strings.Repeat(" ", e.indent)).Replace(text)
}

// reset replaces the text with a fresh history, with the cursor at its end.
func (e *Editor) reset(text string) {
	e.buf = kernel.New(text, nil)
	e.marks, e.before = make(map[rune]kernel.MarkID), make(map[kernel.NodeID]int)
	e.head, e.depth, e.top = kernel.NodeID{}, 0, 0
	e.cursor = len(text)
}

// leave ends what the machine is doing, such as insert mode or a command being typed, and returns to normal mode.
func (e *Editor) leave() {
	e.vim.Cancel()
	for range 3 {
		if e.vim.Mode() == vim.Normal {
			break
		}

		e.vim.Feed("<Esc>") //nolint:errcheck
	}
}

// SetValue replaces the text, with each tab as a level of indentation, and starts its undo history over. The cursor
// goes to the end, and insert mode stays on.
func (e *Editor) SetValue(text string) {
	inserting := e.Inserting()
	e.leave()
	e.reset(e.untab(text))
	e.err = ""
	if inserting {
		e.StartInsert()
	}
}

// StartInsert ends what the machine is doing and starts insert mode at the cursor, or at the end when the cursor is
// there. It does nothing in insert or replace mode.
func (e *Editor) StartInsert() {
	if e.Inserting() {
		return
	}

	e.leave()
	k := vim.Key("i")
	if e.cursor == e.buf.Snapshot().Len() {
		k = "A"
	}

	e.vim.Feed(k) //nolint:errcheck
	e.scroll()
}

// Replace replaces the text from start to end, in bytes, as one change. A cursor after end moves with the text after
// it, and one inside goes to start. It fails when the text would pass the limit.
func (e *Editor) Replace(start, end int, text string) error {
	if err := e.apply([]vim.Edit{{Start: start, End: end, Text: text}}, "replace"); err != nil {
		return err
	}

	switch {
	case e.cursor >= end:
		e.cursor += len(text) - (end - start)
	case e.cursor > start:
		e.cursor = start
	}

	e.scroll()
	return nil
}

// apply makes edits as one transaction, unless the text would pass the limit.
func (e *Editor) apply(edits []vim.Edit, label string) error {
	t := e.buf.Snapshot()
	n := utf8.RuneCountInString(t.Slice(0, t.Len()))
	for _, edit := range edits {
		n += utf8.RuneCountInString(edit.Text) - utf8.RuneCountInString(t.Slice(edit.Start, edit.End))
	}

	if n > e.limit {
		return fmt.Errorf("draft too long, %d / %d", n, e.limit)
	}

	transaction := kernel.Transaction{Base: t.Version(), Origin: origin, Label: label, Time: time.Now()}
	for _, edit := range edits {
		transaction.Edits = append(transaction.Edits, kernel.Edit(edit))
	}

	e.begin()
	err := e.buf.Apply(transaction)
	e.end()
	return err
}

// begin notes the cursor when a new undo step may begin.
func (e *Editor) begin() {
	if e.depth == 0 {
		e.pending = e.cursor
	}
}

// end keeps the noted cursor for the step just made, if there is one.
func (e *Editor) end() {
	if head := e.buf.Head(); head != e.head {
		e.before[head] = e.pending
		e.head = head
	}
}

// Value returns the text.
func (e *Editor) Value() string {
	t := e.buf.Snapshot()
	return t.Slice(0, t.Len())
}

// Cursor returns the cursor as a byte offset into Value.
func (e *Editor) Cursor() int {
	return e.cursor
}

// Inserting reports whether keys type text, in insert or replace mode.
func (e *Editor) Inserting() bool {
	return e.vim.Mode() == vim.Insert || e.vim.Mode() == vim.Replace
}

// Idle reports whether the editor is in normal mode with no command being typed, so esc has nothing to cancel.
func (e *Editor) Idle() bool {
	return e.vim.Idle()
}

// SetWidth sets the columns of text.
func (e *Editor) SetWidth(width int) {
	e.width = width
	e.scroll()
}

// SetMaxRows sets the most rows of text that show. The editor grows with its text up to that many rows, then scrolls.
func (e *Editor) SetMaxRows(rows int) {
	e.maxRows = rows
	e.scroll()
}

// height returns how many rows show for text of n rows.
func (e *Editor) height(n int) int {
	return min(max(n, minRows), e.maxRows)
}

// cols returns the columns rows wrap at. One column stays free for the cursor after a full row.
func (e *Editor) cols() int {
	return e.width - 1
}

// rows returns where each screen row of the text starts.
func (e *Editor) rows() []int {
	t := e.buf.Snapshot()
	var rows []int
	for start := 0; ; {
		rows = append(rows, vim.WordRows(t, start, math.MaxInt, e.cols(), 0, e.indent, nil)...)
		end := vim.LineEnd(t, start)
		if end >= t.Len() {
			return rows
		}

		start = end + 1
	}
}

// rowOf returns the row that holds pos.
func rowOf(rows []int, pos int) int {
	row := 0
	for row+1 < len(rows) && rows[row+1] <= pos {
		row++
	}

	return row
}

// scroll moves top so the cursor's row shows. It leaves no empty rows below the text when earlier rows could fill them.
func (e *Editor) scroll() {
	rows := e.rows()
	at := rowOf(rows, e.cursor)
	h := e.height(len(rows))
	e.top = min(e.top, max(len(rows)-h, 0))
	e.top = min(max(e.top, at-h+1), at)
}

func (e *Editor) View() string {
	t := e.buf.Snapshot()
	rows := e.rows()
	lines := make([]string, e.height(len(rows)))
	switch {
	case t.Len() == 0 && e.Placeholder != "":
		lines[0] = MutedStyle.Render(e.Placeholder)
		if e.focused && e.vim.Mode() != vim.Cmdline {
			lines[0] = cursorMark(e.shape()) + lines[0]
		}
	default:
		cursor := rowOf(rows, e.cursor)
		for i := range lines {
			row := e.top + i
			if row >= len(rows) {
				break
			}

			end := t.Len()
			if row+1 < len(rows) {
				end = rows[row+1]
			}

			lines[i] = e.row(t, rows[row], end, row == cursor)
			// a row that wraps onto the next ends in a marker, in the column kept free for the cursor at a line end
			if end < t.Len() && t.Slice(end-1, end) != "\n" {
				lines[i] = pad(lines[i], e.cols()) + MutedStyle.Render("↩")
			}
		}
	}

	for i, line := range lines {
		lines[i] = pad(ansi.Truncate(line, e.width, ""), e.width)
	}

	style := inputStyle
	if e.focused {
		style = inputFocusedStyle
	}

	below := len(rows) - e.top - len(lines)
	return e.edge("╭", "╮", MutedStyle.Render(e.Title), MutedStyle.Render(more("↑", e.top))) + "\n" +
		style.BorderTop(false).BorderBottom(false).Render(strings.Join(lines, "\n")) + "\n" +
		e.edge("╰", "╯", e.status(), e.info(below))
}

// edge renders the top or bottom border of the box from corner l to corner r, with left and right set into it. It
// leaves out a label without text.
func (e *Editor) edge(l, r, left, right string) string {
	border := lipgloss.NewStyle().Foreground(ColorBorder)
	if e.focused {
		border = border.Foreground(ColorPrimary)
	}

	// between the corners the border spans the text and its padding. It keeps a line at each end, and a space on each
	// side of a label.
	inner := e.width + 2
	if lipgloss.Width(right) > 0 {
		right = " " + ansi.Truncate(right, max(inner-4, 0), "…") + " "
	}

	if lipgloss.Width(left) > 0 {
		left = " " + ansi.Truncate(left, max(inner-4-lipgloss.Width(right), 0), "…") + " "
	}

	fill := max(inner-2-lipgloss.Width(left)-lipgloss.Width(right), 0)
	return border.Render(l+"─") + left + border.Render(strings.Repeat("─", fill)) + right + border.Render("─"+r)
}

// more tells how many rows are hidden in the direction of arrow, or nothing when none are.
func more(arrow string, n int) string {
	if n <= 0 {
		return ""
	}

	return fmt.Sprintf("%s %d more", arrow, n)
}

// pad adds spaces to line up to width columns.
func pad(line string, width int) string {
	return line + strings.Repeat(" ", max(width-lipgloss.Width(line), 0))
}

var (
	cursorStyle    = lipgloss.NewStyle().Reverse(true)
	selectionStyle = lipgloss.NewStyle().Reverse(true).Faint(true)
)

// row renders the text from start to end as one screen row. It draws the cursor when the row holds it, the other
// cursors while a change types into several regions, and the selection.
func (e *Editor) row(t kernel.Snapshot, start, end int, cursor bool) string {
	selection, selecting := e.vim.Selection()
	// col is the display column from the start of the line, counted as vim counts a block selection
	line := vim.LineStart(t, start)
	col := vim.Column(t, line, start, e.indent, nil)
	selected := func(pos int) bool {
		switch {
		case !selecting:
			return false
		case len(selection.Regions) > 0:
			return slices.ContainsFunc(selection.Regions, func(r [2]int) bool { return pos >= r[0] && pos < r[1] })
		}

		return pos >= selection.Start && pos < selection.End && (!selection.Block || col >= selection.Left && col <= selection.Right)
	}

	// the row marks the cursor for the terminal to draw, unless the command line has it. Other cursors are reversed cells.
	main := func(pos int) bool { return e.focused && cursor && pos == e.cursor && e.vim.Mode() != vim.Cmdline }
	others := e.vim.Cursors()
	other := func(pos int) bool { return e.focused && slices.Contains(others, pos) }

	var b strings.Builder
	pos := start
	for g := graphemes.FromString(t.Slice(start, end)); g.Next(); {
		cluster := g.Value()
		if cluster == "\n" {
			break
		}

		w := vim.Column(t, line, pos+len(cluster), e.indent, nil) - col
		if cluster == "\t" {
			cluster = strings.Repeat(" ", w)
		}

		switch {
		case other(pos):
			cluster = cursorStyle.Render(cluster)
		case selected(pos):
			cluster = selectionStyle.Render(cluster)
		}

		if main(pos) {
			cluster = cursorMark(e.shape()) + cluster
		}

		b.WriteString(cluster)
		pos += len(g.Value())
		col += w
	}

	// the end of a line has a cell for the cursor and for a selected line break
	if pos == vim.LineEnd(t, start) {
		switch {
		case main(pos):
			b.WriteString(cursorMark(e.shape()))
		case other(pos):
			b.WriteString(cursorStyle.Render(" "))
		case selected(pos) && pos < t.Len():
			b.WriteString(selectionStyle.Render(" "))
		}
	}

	return b.String()
}

// status renders the left of the bottom border. While the user types a command line, it shows that line. Otherwise it
// shows the mode, the register a macro records into, the keys of a pending command and why the last key failed.
func (e *Editor) status() string {
	if e.vim.Mode() == vim.Cmdline {
		line, at := e.vim.Line(), e.vim.LineCursor()
		if e.focused {
			line = line[:at] + cursorMark(tea.CursorBar) + line[at:]
		}

		return line
	}

	parts := []string{e.modeStyle().Render(e.modeName())}
	if r := e.vim.Recording(); r != 0 {
		parts = append(parts, MutedStyle.Render("@"+string(r)))
	}

	if shown := e.vim.Shown(); shown != "" {
		parts = append(parts, shown)
	}

	switch {
	case e.err != "":
		parts = append(parts, DangerStyle.Render(e.err))
	case e.vim.Message() != "":
		parts = append(parts, MutedStyle.Render(e.vim.Message()))
	}

	return strings.Join(parts, MutedStyle.Render(" · "))
}

// info renders the right of the bottom border: the rows hidden below and Note.
func (e *Editor) info(below int) string {
	var parts []string
	if below > 0 {
		parts = append(parts, more("↓", below))
	}

	if e.Note != "" {
		parts = append(parts, e.Note)
	}

	return MutedStyle.Render(strings.Join(parts, " · "))
}

// modeStyle colors the mode name like a Neovim statusline, and mutes it while the editor is unfocused.
func (e *Editor) modeStyle() lipgloss.Style {
	if !e.focused {
		return MutedStyle
	}

	switch e.vim.Mode() {
	case vim.Insert:
		return AccentStyle
	case vim.Visual:
		return BoldStyle.Foreground(ColorVisual)
	case vim.Replace:
		return DangerStyle.Bold(true)
	}

	return MutedStyle.Bold(true)
}

// shape returns the cursor shape of the mode, as Neovim draws it. It is a bar in insert mode, an underline in replace
// mode and a block otherwise.
func (e *Editor) shape() tea.CursorShape {
	switch e.vim.Mode() {
	case vim.Insert:
		return tea.CursorBar
	case vim.Replace:
		return tea.CursorUnderline
	}

	return tea.CursorBlock
}

// modeName names the mode for the status line.
func (e *Editor) modeName() string {
	switch e.vim.Mode() {
	case vim.Insert:
		return "INSERT"
	case vim.Replace:
		return "REPLACE"
	case vim.Visual:
		selection, _ := e.vim.Selection()
		switch selection.Kind {
		case "V":
			return "V-LINE"
		case "<C-v>":
			return "V-BLOCK"
		}

		return "VISUAL"
	}

	return "NORMAL"
}

// host implements vim.Host for an Editor.
type host struct {
	vim.Defaults
	e *Editor
}

func (h host) Text() vim.Text    { return h.e.buf.Snapshot() }
func (h host) Cursor() int       { return h.e.cursor }
func (h host) SetCursor(pos int) { h.e.cursor = pos }

func (h host) Apply(edits []vim.Edit, label string) error {
	return h.e.apply(edits, label)
}

func (h host) BeginGroup() {
	h.e.begin()
	h.e.depth++
	h.e.buf.BeginGroup(origin)
}

func (h host) EndGroup() {
	h.e.depth--
	h.e.buf.EndGroup(origin)
	h.e.end()
}

func (h host) Undo() (int, int, error) {
	head := h.e.buf.Head()
	start, ok := h.e.buf.Undo()
	if !ok {
		return 0, 0, vim.ErrNoStep
	}

	h.e.head = h.e.buf.Head()
	return start, h.e.before[head], nil
}

func (h host) Redo() (int, int, error) {
	start, ok := h.e.buf.Redo()
	if !ok {
		return 0, 0, vim.ErrNoStep
	}

	h.e.head = h.e.buf.Head()
	return start, h.e.before[h.e.head], nil
}

func (h host) SetMark(name rune, pos int) {
	if id, ok := h.e.marks[name]; ok {
		h.e.buf.MoveMark(id, pos)
		return
	}

	h.e.marks[name] = h.e.buf.SetMark(pos, kernel.Right)
}

func (h host) DeleteMark(name rune) {
	if id, ok := h.e.marks[name]; ok {
		h.e.buf.DeleteMark(id)
		delete(h.e.marks, name)
	}
}

func (h host) Mark(name rune) (int, bool) {
	id, ok := h.e.marks[name]
	if !ok {
		return 0, false
	}

	return h.e.buf.Mark(id), true
}

// Viewport returns the start of the line of the first row shown.
func (h host) Viewport() (int, int, int) {
	t := h.e.buf.Snapshot()
	rows := h.e.rows()
	return vim.LineStart(t, rows[min(h.e.top, len(rows)-1)]), h.e.height(len(rows)), h.e.cols()
}

// SetTop shows the line at top from its first row.
func (h host) SetTop(top int) {
	h.e.top = rowOf(h.e.rows(), vim.LineStart(h.e.buf.Snapshot(), top))
}

func (h host) WordWrap() bool                 { return true }
func (h host) IndentWidth() int               { return h.e.indent }
func (h host) TabWidth() int                  { return h.e.indent }
func (h host) Clipboard() (string, error)     { return clipboard.ReadAll() }
func (h host) SetClipboard(text string) error { return clipboard.WriteAll(text) }
