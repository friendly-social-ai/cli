package ui

import (
	"os/exec"
	"testing"
)

func TestDisplayMath(t *testing.T) {
	tests := []struct {
		name, text string
		want       []string
	}{
		{"block", "before\n$$\n\\frac{a}{b}\n$$\nafter", []string{"\\frac{a}{b}\n"}},
		{"unclosed block runs to the end", "$$\nx^2\ny^2", []string{"x^2\ny^2"}},
		{"one line is inline", "so $$x$$ here", nil},
		{"inside a code block", "```\n$$\nx\n$$\n```", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := DisplayMath(tt.text)
			if len(blocks) != len(tt.want) {
				t.Fatalf("DisplayMath(%q) = %v, want TeX %q", tt.text, blocks, tt.want)
			}

			for i, b := range blocks {
				if b.TeX != tt.want[i] {
					t.Errorf("DisplayMath(%q)[%d].TeX = %q, want %q", tt.text, i, b.TeX, tt.want[i])
				}
			}
		})
	}
}

func TestDisplayMathCoversItsLines(t *testing.T) {
	text := "before\n$$\nx\n$$\nafter"
	b := DisplayMath(text)[0]
	if got := text[b.Start:b.End]; got != "$$\nx\n$$\n" {
		t.Errorf("block covers %q, want the fences and the TeX", got)
	}
}

func TestRenderMathAtTextSize(t *testing.T) {
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("typst is not installed")
	}

	tests := []struct {
		tex  string
		rows int
	}{
		{`x^2 + y^2`, 1},
		{`\frac{a}{b}`, 2},
	}

	for _, tt := range tests {
		img, err := RenderMath(tt.tex)
		if err != nil {
			t.Fatalf("RenderMath(%q) = %v", tt.tex, err)
		}

		if _, rows := MathCells(img, 80); rows != tt.rows {
			t.Errorf("MathCells(RenderMath(%q)) rows = %d, want %d", tt.tex, rows, tt.rows)
		}
	}

	wide, err := RenderMath(`\sum_{i=1}^{n} i = \frac{n(n+1)}{2}`)
	if err != nil {
		t.Fatalf("RenderMath() = %v", err)
	}

	if cols, _ := MathCells(wide, 10); cols > 10 {
		t.Errorf("MathCells(wide, 10) cols = %d, want at most 10", cols)
	}
}

func TestRenderMathUnknownCommandFails(t *testing.T) {
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("typst is not installed")
	}

	if _, err := RenderMath(`\notacommand{x}`); err == nil {
		t.Error("RenderMath() = nil, want an error")
	}
}
