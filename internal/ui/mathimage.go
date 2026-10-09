package ui

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// mathTimeout bounds a typst run, which downloads mitex the first time.
	mathTimeout = 15 * time.Second

	// mathPPI is 3 pixels a point. mathRowPixels is the height of a terminal row in rendered math, about 14pt, the
	// line height of the 11pt text in mathSource.
	mathPPI       = 216
	mathRowPixels = 42
)

// mathSource is the typst document that renders TeX input with mitex, which reads LaTeX math like KaTeX on the web.
// New Computer Modern is the font family KaTeX's own fonts copy. KaTeX's fonts have no OpenType MATH table, so typst
// can't lay out math with them.
const mathSource = `#import "@preview/mitex:0.2.7": mitex
#set page(width: auto, height: auto, margin: (x: 0pt, y: 2pt), fill: none)
#set text(font: "New Computer Modern", size: 11pt, fill: rgb(sys.inputs.color))
#mitex(sys.inputs.tex)
`

// RenderMath renders TeX display math to an image with typst and mitex. The theme has no text color, so math is
// near white on dark terminals and near black on light ones. Typst runs in an empty root folder, since the TeX comes
// from posts and mitex evaluates it as typst code.
func RenderMath(tex string) (image.Image, error) {
	root := filepath.Join(os.TempDir(), "friendly-math")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create typst root: %w", err)
	}

	color := "#1E1E1E"
	if palette.dark {
		color = "#E8E8E8"
	}

	ctx, cancel := context.WithTimeout(context.Background(), mathTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "typst", "compile", "--root", root, "--ignore-system-fonts",
		"--ppi", strconv.Itoa(mathPPI), "--input", "color="+color, "--input", "tex="+tex, "--format", "png", "-", "-")
	cmd.Stdin = strings.NewReader(mathSource)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		// typst prints the error on the first line and its trace through mitex after it
		problem, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
		return nil, fmt.Errorf("failed to render math with typst: %w: %s", err, problem)
	}

	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("failed to decode math image: %w", err)
	}

	return img, nil
}

// MathCells returns the size in cells of math rendered by RenderMath at the size of the text around it, shrunk to
// fit maxCols.
func MathCells(img image.Image, maxCols int) (int, int) {
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return 0, 0
	}

	rows := max((bounds.Dy()+mathRowPixels/2)/mathRowPixels, 1)
	// cells are twice as tall as wide
	cols := max((2*bounds.Dx()*rows+bounds.Dy()/2)/bounds.Dy(), 1)
	if cols > maxCols {
		return Fit(img, maxCols, rows)
	}

	return cols, rows
}
