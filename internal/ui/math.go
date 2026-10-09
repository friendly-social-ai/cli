package ui

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	// fenceOpenPattern matches the opening fence of a code block and captures it and its language.
	fenceOpenPattern = regexp.MustCompile("^ {0,3}(`{3,}|~{3,}) *([^ `]*)")
	// mathOpenPattern matches the opening fence of display math and captures it.
	mathOpenPattern = regexp.MustCompile(`^ {0,3}(\$\$+)[^$]*$`)
	// fenceClosePattern matches a closing fence of a code block or display math and captures it.
	fenceClosePattern = regexp.MustCompile("^ {0,3}(`{3,}|~{3,}|\\$\\$+) *$")
)

// mathText replaces math in markdown text with Unicode. It finds math the way remark-math and rehype-katex do on the
// web: inline between matching runs of $ and display between lines of $$ or in a math code block. Math that Unicode
// can't show stays as its source, inline in a code span and display in a latex code block.
func mathText(text string) string {
	var out strings.Builder
	// para is the start of the paragraph being collected, -1 between paragraphs
	para := -1
	flush := func(end int) {
		if para >= 0 {
			out.WriteString(inlineMath(text[para:end], escapeMarkdown))
			para = -1
		}
	}

	for _, b := range blocks(text) {
		if b.kind == lineBlock {
			if para < 0 {
				para = b.start
			}
			continue
		}

		flush(b.start)
		if b.kind == mathBlock {
			out.WriteString(displayMath(b.tex))
		} else {
			out.WriteString(text[b.start:b.end])
		}
	}
	flush(len(text))

	return out.String()
}

// MathBlock is display math of markdown text: the TeX between lines of $$ or in a math code block, and the bytes of
// text from the opening line to the end of the closing one.
type MathBlock struct {
	TeX        string
	Start, End int
}

// DisplayMath returns the display math of markdown text in order, the blocks the web shows on their own line.
func DisplayMath(text string) []MathBlock {
	var out []MathBlock
	for _, b := range blocks(text) {
		if b.kind == mathBlock {
			out = append(out, MathBlock{TeX: b.tex, Start: b.start, End: b.end})
		}
	}

	return out
}

type blockKind int

const (
	lineBlock blockKind = iota
	blankBlock
	codeBlock
	mathBlock
)

// block is a part of markdown text from byte start to end: a line of a paragraph, a blank line, a code block or
// display math with its TeX.
type block struct {
	kind       blockKind
	start, end int
	tex        string
}

// blocks splits markdown text into lines of paragraphs, blank lines, code blocks and display math, the way
// remark-math and rehype-katex on the web read it. A block without its closing fence runs to the end of text.
func blocks(text string) []block {
	var out []block
	for start := 0; start < len(text); {
		line, end := lineAt(text, start)
		b := block{kind: lineBlock, start: start}
		switch {
		case strings.TrimSpace(line) == "":
			b.kind = blankBlock
		case fenceOpenPattern.MatchString(line):
			m := fenceOpenPattern.FindStringSubmatch(line)
			bodyEnd, next := closeFence(text, end, m[1])
			b.kind = codeBlock
			if m[2] == "math" {
				b.kind, b.tex = mathBlock, text[end:bodyEnd]
			}
			end = next
		case mathOpenPattern.MatchString(line):
			bodyEnd, next := closeFence(text, end, mathOpenPattern.FindStringSubmatch(line)[1])
			b.kind, b.tex, end = mathBlock, text[end:bodyEnd], next
		}

		b.end = end
		out = append(out, b)
		start = end
	}

	return out
}

// lineAt returns the line of text at start without its line break, and the start of the next line.
func lineAt(text string, start int) (string, int) {
	end := len(text)
	if i := strings.IndexByte(text[start:], '\n'); i >= 0 {
		end = start + i + 1
	}

	return strings.TrimRight(text[start:end], "\r\n"), end
}

// closeFence returns the start and end of the line that closes a block opened with fence open, searching from byte
// start of text. Both are the end of text when no line closes it.
func closeFence(text string, start int, open string) (int, int) {
	for start < len(text) {
		line, next := lineAt(text, start)
		if closesFence(line, open) {
			return start, next
		}
		start = next
	}

	return len(text), len(text)
}

// closesFence reports whether line closes a block opened with fence open.
func closesFence(line, open string) bool {
	m := fenceClosePattern.FindStringSubmatch(line)
	return m != nil && m[1][0] == open[0] && len(m[1]) >= len(open)
}

// PlainMath replaces inline math in a line of text with Unicode for plain text, like a preview. Math that Unicode
// can't show stays as its source in a code span.
func PlainMath(line string) string {
	return inlineMath(line, func(math string) string { return math })
}

// inlineMath replaces inline math in paragraph text with Unicode passed through show, skipping escapes and code
// spans.
func inlineMath(s string, show func(string) string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			// glamour doesn't unescape \$, so it becomes a plain $ here
			escape := s[i : i+2]
			if escape == `\$` {
				escape = "$"
			}
			b.WriteString(escape)
			i += 2
			continue
		}

		if c != '`' && c != '$' {
			b.WriteByte(c)
			i++
			continue
		}

		n := runLength(s[i:], c)
		end := closingRun(s, i+n, c, n)
		switch {
		case end < 0:
			b.WriteString(s[i : i+n])
			i += n
			continue
		case c == '`':
			b.WriteString(s[i : end+n])
		default:
			src := s[i : end+n]
			if math, ok := unicodeMath(s[i+n : end]); ok {
				b.WriteString(show(math))
			} else {
				fence := backtickFence(src, 1)
				b.WriteString(fence)
				b.WriteString(src)
				b.WriteString(fence)
			}
		}
		i = end + n
	}

	return b.String()
}

// closingRun returns the index of the first run of exactly n c in s from start, or -1.
func closingRun(s string, start int, c byte, n int) int {
	for i := start; i < len(s); {
		if s[i] != c {
			i++
			continue
		}

		m := runLength(s[i:], c)
		if m == n {
			return i
		}
		i += m
	}

	return -1
}

// runLength returns the number of c that s starts with.
func runLength(s string, c byte) int {
	return len(s) - len(strings.TrimLeft(s, string(c)))
}

// displayMath returns display math body as a line of Unicode, or as a latex code block when Unicode can't show it.
func displayMath(body string) string {
	if math, ok := unicodeMath(body); ok {
		return escapeMarkdown(math) + "\n"
	}

	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	fence := backtickFence(body, 3)

	return fence + "latex\n" + body + fence + "\n"
}

// backtickFence returns a run of backticks longer than any in s and no shorter than shortest.
func backtickFence(s string, shortest int) string {
	n := shortest
	for run := range strings.FieldsFuncSeq(s, func(r rune) bool { return r != '`' }) {
		n = max(n, len(run)+1)
	}

	return strings.Repeat("`", n)
}

// escapeMarkdown escapes the markdown punctuation of s that glamour unescapes, so markdown shows it as it is.
func escapeMarkdown(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune("\\`*_{}[]<>()#+-.!|", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}

	return b.String()
}

// unicodeMath returns TeX math src as one line of Unicode, or false when it uses something Unicode can't show.
func unicodeMath(src string) (string, bool) {
	p := &mathParser{src: []rune(src)}
	out := p.list(false)

	return strings.TrimSpace(out), !p.failed
}

// mathParser converts TeX math to Unicode. It sets failed on anything Unicode can't show in one line.
type mathParser struct {
	src    []rune
	pos    int
	failed bool
}

// list converts atoms up to the end of src, or up to the closing brace when inGroup.
func (p *mathParser) list(inGroup bool) string {
	var b strings.Builder
	for p.pos < len(p.src) && !p.failed {
		switch r := p.src[p.pos]; {
		case r == '}':
			p.pos++
			p.failed = !inGroup
			return b.String()
		case unicode.IsSpace(r):
			p.pos++
			if b.Len() > 0 && !strings.HasSuffix(b.String(), " ") {
				b.WriteByte(' ')
			}
		case r == '^' || r == '_':
			p.pos++
			b.WriteString(p.script(r))
		default:
			b.WriteString(p.atom())
		}
	}
	p.failed = p.failed || inGroup

	return b.String()
}

// atom converts one character, group or command.
func (p *mathParser) atom() string {
	r := p.src[p.pos]
	p.pos++
	switch r {
	case '{':
		return p.list(true)
	case '\\':
		return p.command()
	case '\'':
		return "′"
	case '~':
		return " "
	case '&', '#', '%':
		p.failed = true
		return ""
	}

	return string(r)
}

// arg converts the argument of a command or script: a group, a command or one character.
func (p *mathParser) arg() string {
	for p.pos < len(p.src) && unicode.IsSpace(p.src[p.pos]) {
		p.pos++
	}

	if p.pos == len(p.src) || p.src[p.pos] == '}' {
		p.failed = true
		return ""
	}

	return p.atom()
}

// script converts the argument of ^ or _ to superscript or subscript characters. An argument without them shows
// after the mark, in parentheses when longer than one character.
func (p *mathParser) script(mark rune) string {
	arg := strings.ReplaceAll(p.arg(), " ", "")
	table := superscripts
	if mark == '_' {
		table = subscripts
	}

	if out, ok := mapRunes(arg, func(r rune) (rune, bool) { s, ok := table[r]; return s, ok }); ok {
		return out
	}

	if utf8.RuneCountInString(arg) == 1 {
		return string(mark) + arg
	}

	return string(mark) + "(" + arg + ")"
}

// command converts the command after a backslash.
func (p *mathParser) command() string {
	if p.pos == len(p.src) {
		p.failed = true
		return ""
	}

	start := p.pos
	if r := p.src[p.pos]; !isASCIILetter(r) {
		p.pos++
		switch r {
		case ',', ':', ';', ' ':
			return " "
		case '!':
			return ""
		case '{', '}', '$', '%', '#', '&', '_':
			return string(r)
		case '|':
			return "‖"
		}

		p.failed = true
		return ""
	}

	for p.pos < len(p.src) && isASCIILetter(p.src[p.pos]) {
		p.pos++
	}

	name := string(p.src[start:p.pos])
	if s, ok := mathSymbols[name]; ok {
		return s
	}

	if accent, ok := mathAccents[name]; ok {
		arg := p.arg()
		p.failed = p.failed || utf8.RuneCountInString(arg) != 1
		return arg + accent
	}

	switch name {
	case "frac", "dfrac", "tfrac":
		num := p.arg()
		return parenthesize(num) + "/" + parenthesize(p.arg())
	case "sqrt":
		return p.root()
	case "mathbb":
		out, ok := mapRunes(p.arg(), doubleStruck)
		p.failed = p.failed || !ok
		return out
	case "text", "textrm", "textit", "textbf", "mathrm", "mathit", "mathbf", "mathsf", "mathtt", "mathcal",
		"mathscr", "mathfrak", "boldsymbol", "operatorname":
		return p.arg()
	case "left", "right", "big", "Big", "bigg", "Bigg", "bigl", "bigr", "Bigl", "Bigr", "biggl", "biggr":
		if p.pos < len(p.src) && p.src[p.pos] == '.' {
			p.pos++
		}
		return ""
	case "displaystyle", "textstyle", "limits", "nolimits":
		return ""
	}

	p.failed = true
	return ""
}

// root converts the argument of \sqrt, with an optional index of 3 or 4.
func (p *mathParser) root() string {
	sign := "√"
	if p.pos < len(p.src) && p.src[p.pos] == '[' {
		end := p.pos + 1
		for end < len(p.src) && p.src[end] != ']' {
			end++
		}

		switch strings.TrimSpace(string(p.src[p.pos+1 : min(end, len(p.src))])) {
		case "3":
			sign = "∛"
		case "4":
			sign = "∜"
		default:
			p.failed = true
		}
		p.pos = end + 1
	}

	return sign + parenthesize(p.arg())
}

// parenthesize wraps s in parentheses unless it is one character or a number.
func parenthesize(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= 1 || strings.Trim(s, "0123456789") == "" {
		return s
	}

	return "(" + s + ")"
}

func isASCIILetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// mapRunes maps every rune of s with f, or returns false when f can't map one.
func mapRunes(s string, f func(rune) (rune, bool)) (string, bool) {
	var b strings.Builder
	for _, r := range s {
		m, ok := f(r)
		if !ok {
			return s, false
		}
		b.WriteRune(m)
	}

	return b.String(), true
}

// doubleStruck maps letters and digits to their double-struck form, as \mathbb shows them.
func doubleStruck(r rune) (rune, bool) {
	if m, ok := doubleStruckCapitals[r]; ok {
		return m, true
	}

	switch {
	case r >= 'A' && r <= 'Z':
		return 0x1D538 + r - 'A', true
	case r >= 'a' && r <= 'z':
		return 0x1D552 + r - 'a', true
	case r >= '0' && r <= '9':
		return 0x1D7D8 + r - '0', true
	}

	return r, false
}

// doubleStruckCapitals holds the double-struck capitals that sit in the Letterlike Symbols block.
var doubleStruckCapitals = map[rune]rune{'C': 'ℂ', 'H': 'ℍ', 'N': 'ℕ', 'P': 'ℙ', 'Q': 'ℚ', 'R': 'ℝ', 'Z': 'ℤ'}

var (
	superscripts = zipRunes(
		"0123456789+-=()abcdefghijklmnoprstuvwxyzABDEGHIJKLMNOPRTUVWβγδθιφχ′∘",
		"⁰¹²³⁴⁵⁶⁷⁸⁹⁺⁻⁼⁽⁾ᵃᵇᶜᵈᵉᶠᵍʰⁱʲᵏˡᵐⁿᵒᵖʳˢᵗᵘᵛʷˣʸᶻᴬᴮᴰᴱᴳᴴᴵᴶᴷᴸᴹᴺᴼᴾᴿᵀᵁⱽᵂᵝᵞᵟᶿᶥᵠᵡ′°")
	subscripts = zipRunes(
		"0123456789+-=()aehijklmnoprstuvxβγρφχ",
		"₀₁₂₃₄₅₆₇₈₉₊₋₌₍₎ₐₑₕᵢⱼₖₗₘₙₒₚᵣₛₜᵤᵥₓᵦᵧᵨᵩᵪ")
)

// zipRunes maps each rune of from to the rune of to at the same position.
func zipRunes(from, to string) map[rune]rune {
	f, t := []rune(from), []rune(to)
	if len(f) != len(t) {
		panic("zipRunes: " + from + " and " + to + " differ in length")
	}

	m := make(map[rune]rune, len(f))
	for i, r := range f {
		m[r] = t[i]
	}

	return m
}

// mathAccents maps accent commands to the combining character they put on one letter.
var mathAccents = map[string]string{
	"hat": "̂", "bar": "̄", "overline": "̅", "vec": "⃗", "dot": "̇", "ddot": "̈",
	"tilde": "̃",
}

// mathSymbols maps TeX commands to the Unicode they show as.
var mathSymbols = map[string]string{
	// greek
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ϵ", "varepsilon": "ε", "zeta": "ζ",
	"eta": "η", "theta": "θ", "vartheta": "ϑ", "iota": "ι", "kappa": "κ", "lambda": "λ", "mu": "μ", "nu": "ν",
	"xi": "ξ", "pi": "π", "varpi": "ϖ", "rho": "ρ", "varrho": "ϱ", "sigma": "σ", "varsigma": "ς", "tau": "τ",
	"upsilon": "υ", "phi": "ϕ", "varphi": "φ", "chi": "χ", "psi": "ψ", "omega": "ω",
	"Gamma": "Γ", "Delta": "Δ", "Theta": "Θ", "Lambda": "Λ", "Xi": "Ξ", "Pi": "Π", "Sigma": "Σ", "Upsilon": "Υ",
	"Phi": "Φ", "Psi": "Ψ", "Omega": "Ω",
	// operators
	"times": "×", "cdot": "⋅", "div": "÷", "pm": "±", "mp": "∓", "ast": "∗", "star": "⋆", "circ": "∘",
	"bullet": "∙", "cap": "∩", "cup": "∪", "wedge": "∧", "land": "∧", "vee": "∨", "lor": "∨", "oplus": "⊕",
	"otimes": "⊗", "setminus": "∖", "bmod": "mod",
	// relations
	"leq": "≤", "le": "≤", "geq": "≥", "ge": "≥", "neq": "≠", "ne": "≠", "ll": "≪", "gg": "≫", "approx": "≈",
	"equiv": "≡", "sim": "∼", "simeq": "≃", "cong": "≅", "propto": "∝", "in": "∈", "notin": "∉", "ni": "∋",
	"subset": "⊂", "supset": "⊃", "subseteq": "⊆", "supseteq": "⊇", "mid": "∣", "parallel": "∥", "perp": "⊥",
	"vdash": "⊢", "models": "⊨", "coloneqq": "≔",
	// arrows
	"to": "→", "rightarrow": "→", "leftarrow": "←", "gets": "←", "leftrightarrow": "↔", "Rightarrow": "⇒",
	"Leftarrow": "⇐", "Leftrightarrow": "⇔", "implies": "⟹", "iff": "⟺", "mapsto": "↦", "uparrow": "↑",
	"downarrow": "↓", "longrightarrow": "⟶", "longleftarrow": "⟵", "hookrightarrow": "↪",
	// big operators
	"sum": "∑", "prod": "∏", "coprod": "∐", "int": "∫", "iint": "∬", "iiint": "∭", "oint": "∮", "bigcup": "⋃",
	"bigcap": "⋂", "bigoplus": "⨁", "bigotimes": "⨂",
	// functions
	"sin": "sin", "cos": "cos", "tan": "tan", "cot": "cot", "sec": "sec", "csc": "csc", "arcsin": "arcsin",
	"arccos": "arccos", "arctan": "arctan", "sinh": "sinh", "cosh": "cosh", "tanh": "tanh", "log": "log",
	"ln": "ln", "lg": "lg", "exp": "exp", "lim": "lim", "limsup": "lim sup", "liminf": "lim inf", "max": "max",
	"min": "min", "sup": "sup", "inf": "inf", "det": "det", "gcd": "gcd", "deg": "deg", "dim": "dim", "ker": "ker",
	"arg": "arg", "hom": "hom", "Pr": "Pr",
	// delimiters
	"langle": "⟨", "rangle": "⟩", "lfloor": "⌊", "rfloor": "⌋", "lceil": "⌈", "rceil": "⌉", "vert": "|",
	"lvert": "|", "rvert": "|", "Vert": "‖", "lVert": "‖", "rVert": "‖", "backslash": "\\",
	// misc
	"infty": "∞", "partial": "∂", "nabla": "∇", "forall": "∀", "exists": "∃", "nexists": "∄", "emptyset": "∅",
	"varnothing": "∅", "neg": "¬", "lnot": "¬", "angle": "∠", "triangle": "△", "hbar": "ℏ", "ell": "ℓ",
	"aleph": "ℵ", "Re": "ℜ", "Im": "ℑ", "wp": "℘", "top": "⊤", "bot": "⊥", "prime": "′", "dagger": "†",
	"therefore": "∴", "because": "∵", "cdots": "⋯", "ldots": "…", "dots": "…", "vdots": "⋮", "ddots": "⋱",
	// spacing
	"quad": " ", "qquad": "  ",
}
