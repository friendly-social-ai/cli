package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestMarkdownRendersMath(t *testing.T) {
	tests := []struct {
		name, text, want string
	}{
		{"greek and superscript", `$\alpha + \beta^2$`, "α + β²"},
		{"subscript and root", `$x_{i} \leq \sqrt{n}$`, "xᵢ ≤ √n"},
		{"fraction", `$\frac{a+b}{2}$`, "(a+b)/2"},
		{"blackboard bold", `$x \in \mathbb{R}$`, "x ∈ ℝ"},
		{"script without unicode form", `$e^{i\pi}$`, "e^(iπ)"},
		{"markdown marks inside math", `$a*b*c + x_1$`, "a*b*c + x₁"},
		{"double dollars inline", `so $$a \neq b$$ here`, "so a ≠ b here"},
		{"unsupported inline shows source", `see $\begin{matrix}a\end{matrix}$`, `$\begin{matrix}a\end{matrix}$`},
		{"code span", "`$\\alpha$`", `$\alpha$`},
		{"code block", "```\n$\\alpha$\n```", `$\alpha$`},
		{"escaped dollars", `\$\alpha\$`, `$\alpha$`},
		{"unclosed dollar", "costs $5", "costs $5"},
		{"display math", "$$\n\\sum_{i=1}^n i\n$$", "∑ᵢ₌₁ⁿ i"},
		{"unsupported display shows source", "$$\na \\\\ b\n$$", `a \\ b`},
	}

	m := NewMarkdown()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ansi.Strip(m.Render(tt.text, 80)); !strings.Contains(got, tt.want) {
				t.Errorf("Render(%q) = %q, want it to contain %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestMarkdownMathStopsAtParagraph(t *testing.T) {
	got := ansi.Strip(NewMarkdown().Render("costs $5\n\nand $10", 80))
	if !strings.Contains(got, "costs $5") || !strings.Contains(got, "and $10") {
		t.Errorf("Render() = %q, want both dollars kept", got)
	}
}

func TestPlainMathKeepsPunctuationUnescaped(t *testing.T) {
	if got, want := PlainMath(`$\frac{a+b}{2}$ for \$5`), "(a+b)/2 for $5"; got != want {
		t.Errorf("PlainMath() = %q, want %q", got, want)
	}
}
