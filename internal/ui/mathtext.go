package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/fontscan"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// textSize is the size text is shaped at, a thousand units an em like MathJax.
const textSize = 1000

// textFonts shapes math text that the math font lacks with system fonts. It picks a font for each character, as a
// browser does for MathJax on the web. fontscan scans the system fonts once per process and caches the scan on disk
// for later runs.
//
// Each render gets its own textFonts and drops it after. The font map keeps every face it loads in memory, and a
// CJK or emoji face holds tens to hundreds of megabytes of tables.
type textFonts struct {
	fonts  *fontscan.FontMap
	shaped map[textKey]*shapedText
	images map[glyphKey]image.Image
}

// textKey is text in a font family and style.
type textKey struct {
	text, family string
	italic, bold bool
}

type glyphKey struct {
	face *font.Face
	id   font.GID
}

// shapedText is text shaped at textSize, its runs in visual order. ascent and descent measure its ink above and
// below the baseline.
type shapedText struct {
	runs                   []shaping.Output
	width, ascent, descent float64
}

// silentLogger drops the messages of fontscan, which would print over the app.
type silentLogger struct{}

func (silentLogger) Printf(string, ...any) {}

// shape shapes text with the system fonts that have its characters, preferring the family and style of key.
func (t *textFonts) shape(key textKey) (*shapedText, error) {
	if t.fonts == nil {
		fonts := fontscan.NewFontMap(silentLogger{})
		// an empty cache folder lets fontscan pick one
		cache, err := os.UserCacheDir()
		if err == nil {
			cache = filepath.Join(cache, "friendly", "fonts")
		}
		if err := fonts.UseSystemFonts(cache); err != nil {
			return nil, fmt.Errorf("failed to load system fonts: %w", err)
		}
		t.fonts, t.shaped, t.images = fonts, map[textKey]*shapedText{}, map[glyphKey]image.Image{}
	}

	if s, ok := t.shaped[key]; ok {
		return s, nil
	}

	aspect := font.Aspect{Style: font.StyleNormal, Weight: font.WeightNormal}
	if key.italic {
		aspect.Style = font.StyleItalic
	}
	if key.bold {
		aspect.Weight = font.WeightBold
	}
	t.fonts.SetQuery(fontscan.Query{Families: []string{key.family}, Aspect: aspect})

	text := []rune(key.text)
	input := shaping.Input{Text: text, RunEnd: len(text), Direction: di.DirectionLTR, Size: fixed.I(textSize)}
	var segmenter shaping.Segmenter
	var shaper shaping.HarfbuzzShaper
	var runs []shaping.Output
	for _, in := range segmenter.Split(input, t.fonts) {
		// fontscan finds no face only when the system has no fonts
		if in.Face == nil {
			return nil, fmt.Errorf("no system font has %q", string(text[in.RunStart:in.RunEnd]))
		}

		out := shaper.Shape(in)
		for _, g := range out.Glyphs {
			// glyph 0 is the box a font draws for a character it lacks
			if g.GlyphID == 0 {
				return nil, fmt.Errorf("no system font has %q", text[g.TextIndex()])
			}
		}
		runs = append(runs, out)
	}

	// the line wrapper orders runs of right to left text among left to right text
	var wrapper shaping.LineWrapper
	config := shaping.WrapConfig{Direction: di.DirectionLTR, DisableTrailingWhitespaceTrim: true}
	lines, _ := wrapper.WrapParagraph(config, math.MaxInt32, text, shaping.NewSliceIterator(runs))
	s := &shapedText{}
	for _, line := range lines {
		s.runs = append(s.runs, line...)
	}
	slices.SortFunc(s.runs, func(a, b shaping.Output) int { return int(a.VisualIndex - b.VisualIndex) })

	for _, r := range s.runs {
		s.width += float64(r.Advance) / 64
		s.ascent = max(s.ascent, float64(r.GlyphBounds.Ascent)/64)
		s.descent = max(s.descent, -float64(r.GlyphBounds.Descent)/64)
	}
	t.shaped[key] = s

	return s, nil
}

// glyphs calls outline for each vector glyph of s, and picture for each bitmap glyph, which emoji fonts hold. Both
// get a matrix into text units, where the baseline starts at the origin and y grows down. The matrix of outline maps
// the units of its font, and the matrix of picture maps the unit square to the box of the glyph.
func (t *textFonts) glyphs(s *shapedText, outline func(o font.GlyphOutline, m affine), picture func(img image.Image, m affine)) error {
	pen := 0.0
	for _, r := range s.runs {
		scale := textSize / float64(r.Face.Upem())
		for _, g := range r.Glyphs {
			x, y := pen+float64(g.XOffset)/64, -float64(g.YOffset)/64
			pen += float64(g.Advance) / 64

			switch data := r.Face.GlyphData(g.GlyphID).(type) {
			case font.GlyphOutline:
				outline(data, affine{scale, 0, 0, -scale, x, y})
			case font.GlyphBitmap:
				img, err := t.image(r.Face, g.GlyphID, data)
				if err != nil {
					return err
				}
				w, h := float64(g.Width)/64, -float64(g.Height)/64
				picture(img, affine{w, 0, 0, h, x + float64(g.XBearing)/64, y - float64(g.YBearing)/64})
			case nil:
			default:
				return fmt.Errorf("glyph %d of %s has unsupported data %T", g.GlyphID, r.Face.Describe().Family, data)
			}
		}
	}

	return nil
}

// image decodes the bitmap of a glyph.
func (t *textFonts) image(face *font.Face, id font.GID, b font.GlyphBitmap) (image.Image, error) {
	key := glyphKey{face, id}
	if img, ok := t.images[key]; ok {
		return img, nil
	}

	var img image.Image
	var err error
	switch b.Format {
	case font.PNG:
		img, err = png.Decode(bytes.NewReader(b.Data))
	case font.JPG:
		img, err = jpeg.Decode(bytes.NewReader(b.Data))
	default:
		err = fmt.Errorf("unsupported format %d", b.Format)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to decode glyph %d of %s: %w", id, face.Describe().Family, err)
	}
	t.images[key] = img

	return img, nil
}

// outline adds glyph outline o, mapped by m.
func (s *shape) outline(o font.GlyphOutline, m affine) {
	for _, seg := range o.Segments {
		pts := make([]point, 0, 3)
		for _, a := range seg.ArgsSlice() {
			pts = append(pts, m.apply(point{float64(a.X), float64(a.Y)}))
		}

		switch seg.Op {
		case ot.SegmentOpMoveTo:
			s.add('M', pts...)
		case ot.SegmentOpLineTo:
			s.add('L', pts...)
		case ot.SegmentOpQuadTo:
			s.add('Q', pts...)
		case ot.SegmentOpCubeTo:
			s.add('C', pts...)
		}
	}
}
