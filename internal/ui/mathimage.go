package ui

import (
	"bytes"
	"compress/gzip"
	"embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"io/fs"
	"path"
	"runtime/debug"
	"sync"

	"github.com/dop251/goja"
)

//go:generate go run gen_mathjax.go

const (
	// mathEmPixels is the em of rendered math, 11pt at 3 pixels a point. mathRowPixels is the height of a terminal row
	// in rendered math, about 14pt, the line height of 11pt text. mathMarginPixels pads math above and below.
	mathEmPixels     = 33
	mathRowPixels    = 42
	mathMarginPixels = 6
)

// mathjaxFiles holds MathJax bundled by gen_mathjax.go and the glyph ranges of its font, gzipped. MathJax reads LaTeX
// math like KaTeX on the web, and its New Computer Modern font copies the Computer Modern of KaTeX's fonts.
//
//go:embed mathjax
var mathjaxFiles embed.FS

// mathjax runs MathJax, loaded on the first render. Its runtime isn't safe for concurrent use, so RenderMath holds
// mu.
var mathjax struct {
	once   sync.Once
	mu     sync.Mutex
	render func(tex string) (string, error)
	err    error
	// text holds the system fonts of the render in progress, for measureText
	text *textFonts
}

// RenderMath renders TeX display math to an image with MathJax. The theme has no text color, so math is near white
// on dark terminals and near black on light ones.
func RenderMath(tex string) (image.Image, error) {
	mathjax.once.Do(func() { mathjax.err = loadMathJax() })
	if mathjax.err != nil {
		return nil, mathjax.err
	}

	mathjax.mu.Lock()
	defer mathjax.mu.Unlock()

	text := &textFonts{}
	mathjax.text = text
	defer func() {
		loaded := text.fonts != nil
		mathjax.text, *text = nil, textFonts{}
		// system font faces can take hundreds of megabytes, so return them to the OS now, not after the next collection
		if loaded {
			debug.FreeOSMemory()
		}
	}()

	svg, err := mathjax.render(tex)
	if err != nil {
		return nil, fmt.Errorf("failed to render math with mathjax: %w", err)
	}

	col := color.RGBA{0x1E, 0x1E, 0x1E, 0xFF}
	if palette.dark {
		col = color.RGBA{0xE8, 0xE8, 0xE8, 0xFF}
	}

	// MathJax measures in thousandths of an em
	return drawSVG(svg, col, mathEmPixels/1000.0, mathMarginPixels, text)
}

// loadMathJax runs the MathJax bundle and gives it the functions its entry in gen_mathjax.go calls: loadFontFile,
// which runs a glyph range of the font, and measureText, which measures text the font lacks.
func loadMathJax() error {
	vm := goja.New()
	err := vm.Set("loadFontFile", func(file string) error {
		src, err := gunzip(path.Join("mathjax", "fonts", path.Base(file)+".gz"))
		// gen_mathjax.go leaves out ranges that system fonts shape
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}

		_, err = vm.RunScript(file, src)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to load mathjax: %w", err)
	}

	err = vm.Set("measureText", func(text, family string, italic, bold bool) ([]float64, error) {
		s, err := mathjax.text.shape(textKey{text, family, italic, bold})
		if err != nil {
			return nil, err
		}

		return []float64{s.width / textSize, s.ascent / textSize, s.descent / textSize}, nil
	})
	if err != nil {
		return fmt.Errorf("failed to load mathjax: %w", err)
	}

	src, err := gunzip("mathjax/mathjax.js.gz")
	if err != nil {
		return err
	}

	if _, err := vm.RunScript("mathjax.js", src); err != nil {
		return fmt.Errorf("failed to load mathjax: %w", err)
	}

	if err := vm.ExportTo(vm.Get("render"), &mathjax.render); err != nil {
		return fmt.Errorf("failed to load mathjax: %w", err)
	}

	return nil
}

// gunzip returns the contents of gzipped file name of mathjaxFiles.
func gunzip(name string) (string, error) {
	data, err := mathjaxFiles.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", name, err)
	}

	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", name, err)
	}

	src, err := io.ReadAll(zr)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", name, err)
	}

	return string(src), nil
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
