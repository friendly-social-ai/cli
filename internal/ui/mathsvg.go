package ui

import (
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-text/typesetting/font"
	"golang.org/x/image/colornames"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/vector"
)

var (
	// transformPattern matches a function of an SVG transform attribute and captures its name and arguments.
	transformPattern = regexp.MustCompile(`(\w+)\(([^)]*)\)`)
	// pathTokenPattern matches a command or number of SVG path data.
	pathTokenPattern = regexp.MustCompile(`[A-Za-z]|[-+]?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?`)
)

// tableLineWidth is the width of table lines and frames, which MathJax sets in its stylesheet instead of the SVG.
const tableLineWidth = 70

type point struct{ x, y float64 }

// affine is the SVG transform matrix(a, b, c, d, e, f), which maps (x, y) to (ax+cy+e, bx+dy+f).
type affine [6]float64

var identity = affine{1, 0, 0, 1, 0, 0}

func (m affine) mul(n affine) affine {
	return affine{
		m[0]*n[0] + m[2]*n[1], m[1]*n[0] + m[3]*n[1],
		m[0]*n[2] + m[2]*n[3], m[1]*n[2] + m[3]*n[3],
		m[0]*n[4] + m[2]*n[5] + m[4], m[1]*n[4] + m[3]*n[5] + m[5],
	}
}

func (m affine) apply(p point) point {
	return point{m[0]*p.x + m[2]*p.y + m[4], m[1]*p.x + m[3]*p.y + m[5]}
}

// box is a rectangle from min to max.
type box struct{ min, max point }

var (
	everywhere = box{point{math.Inf(-1), math.Inf(-1)}, point{math.Inf(1), math.Inf(1)}}
	nowhere    = box{point{math.Inf(1), math.Inf(1)}, point{math.Inf(-1), math.Inf(-1)}}
)

func (b box) intersect(c box) box {
	return box{
		point{max(b.min.x, c.min.x), max(b.min.y, c.min.y)},
		point{min(b.max.x, c.max.x), min(b.max.y, c.max.y)},
	}
}

func (b box) union(c box) box {
	return box{
		point{min(b.min.x, c.min.x), min(b.min.y, c.min.y)},
		point{max(b.max.x, c.max.x), max(b.max.y, c.max.y)},
	}
}

func (b box) empty() bool {
	return b.min.x >= b.max.x || b.min.y >= b.max.y
}

// shape is an outline filled with col and clipped to clip, in user units of the root SVG. cmds holds M, L, Q, C and
// Z, which take 1, 1, 2, 3 and 0 of pts.
type shape struct {
	cmds []byte
	pts  []point
	col  color.RGBA
	clip box
}

func (s *shape) add(cmd byte, pts ...point) {
	s.cmds = append(s.cmds, cmd)
	s.pts = append(s.pts, pts...)
}

// polygon adds the closed outline through pts.
func (s *shape) polygon(pts ...point) {
	s.add('M', pts[0])
	for _, p := range pts[1:] {
		s.add('L', p)
	}
	s.add('Z')
}

// bounds returns the box of the points of s inside its clip. Control points can stick out of a curve, so it may be a
// little larger than the ink.
func (s *shape) bounds() box {
	b := nowhere
	for _, p := range s.pts {
		b = b.union(box{p, p})
	}

	return b.intersect(s.clip)
}

// picture is an image drawn over box, in user units of the root SVG.
type picture struct {
	img image.Image
	box box
}

// svgFrame is what an SVG element passes to its children.
type svgFrame struct {
	m            affine
	clip         box
	fill, stroke string
	strokeWidth  float64
}

// drawSVG draws the SVG that MathJax writes, cropped to its ink with margin pixels above and below, at scale pixels a
// user unit. currentColor is col. It reads the elements MathJax writes for math: g, path, rect, line, polygon, text and
// nested svg. It draws text with system fonts and clips to each nested svg.
func drawSVG(src string, col color.RGBA, scale, margin float64) (image.Image, error) {
	shapes, pictures, err := svgShapes(src, col)
	if err != nil {
		return nil, err
	}

	ink := nowhere
	for _, s := range shapes {
		if b := s.bounds(); !b.empty() {
			ink = ink.union(b)
		}
	}
	for _, p := range pictures {
		ink = ink.union(p.box)
	}
	if ink.empty() {
		return nil, errors.New("math has nothing to draw")
	}

	toPixels := affine{scale, 0, 0, scale, -ink.min.x * scale, -ink.min.y*scale + margin}
	w := int(math.Ceil((ink.max.x - ink.min.x) * scale))
	h := int(math.Ceil((ink.max.y-ink.min.y)*scale + 2*margin))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var r vector.Rasterizer
	for _, s := range shapes {
		clip := img.Bounds()
		if s.clip != everywhere {
			lo, hi := toPixels.apply(s.clip.min), toPixels.apply(s.clip.max)
			clip = clip.Intersect(image.Rect(int(math.Floor(lo.x)), int(math.Floor(lo.y)), int(math.Ceil(hi.x)), int(math.Ceil(hi.y))))
		}
		if clip.Empty() {
			continue
		}

		// the rasterizer covers the clip, since Draw puts its origin at the corner of the clip
		r.Reset(clip.Dx(), clip.Dy())
		at := toPixels
		at[4] -= float64(clip.Min.X)
		at[5] -= float64(clip.Min.Y)
		pts := s.pts
		p := func(i int) (float32, float32) {
			q := at.apply(pts[i])
			return float32(q.x), float32(q.y)
		}
		for i, cmd := range s.cmds {
			switch cmd {
			case 'M':
				// font outlines leave their subpaths open
				if i > 0 {
					r.ClosePath()
				}
				r.MoveTo(p(0))
				pts = pts[1:]
			case 'L':
				r.LineTo(p(0))
				pts = pts[1:]
			case 'Q':
				x1, y1 := p(0)
				x, y := p(1)
				r.QuadTo(x1, y1, x, y)
				pts = pts[2:]
			case 'C':
				x1, y1 := p(0)
				x2, y2 := p(1)
				x, y := p(2)
				r.CubeTo(x1, y1, x2, y2, x, y)
				pts = pts[3:]
			case 'Z':
				r.ClosePath()
			}
		}
		r.ClosePath()
		r.Draw(img, clip, image.NewUniform(s.col), image.Point{})
	}

	for _, p := range pictures {
		lo, hi := toPixels.apply(p.box.min), toPixels.apply(p.box.max)
		rect := image.Rect(int(math.Round(lo.x)), int(math.Round(lo.y)), int(math.Round(hi.x)), int(math.Round(hi.y)))
		xdraw.CatmullRom.Scale(img, rect, p.img, p.img.Bounds(), xdraw.Over, nil)
	}

	return img, nil
}

// svgShapes returns the filled and stroked shapes and the pictures of SVG src in user units of its root.
func svgShapes(src string, col color.RGBA) ([]shape, []picture, error) {
	var shapes []shape
	var pictures []picture
	stack := []svgFrame{{m: identity, clip: everywhere, fill: "currentColor", stroke: "none"}}
	dec := xml.NewDecoder(strings.NewReader(src))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return shapes, pictures, nil
		} else if err != nil {
			return nil, nil, fmt.Errorf("failed to read math svg: %w", err)
		}

		if _, ok := tok.(xml.EndElement); ok {
			stack = stack[:len(stack)-1]
			continue
		}

		el, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		attr := map[string]string{}
		for _, a := range el.Attr {
			attr[a.Name.Local] = a.Value
		}

		f := stack[len(stack)-1]
		m, err := transform(attr["transform"])
		if err != nil {
			return nil, nil, err
		}
		f.m = f.m.mul(m)
		if v, ok := attr["fill"]; ok {
			f.fill = v
		}
		if v, ok := attr["stroke"]; ok {
			f.stroke = v
		}
		if v, ok := attr["stroke-width"]; ok {
			f.strokeWidth = number(v)
		} else if _, ok := attr["data-line"]; ok {
			f.strokeWidth = tableLineWidth
		} else if _, ok := attr["data-frame"]; ok {
			f.fill, f.strokeWidth = "none", tableLineWidth
		}

		// fill and stroke add the outline that draw builds in user units of the element, unless its paint is none
		fill := func(draw func(s *shape)) {
			if c, ok := paint(f.fill, col); ok {
				s := shape{col: c, clip: f.clip}
				draw(&s)
				shapes = append(shapes, transformed(s, f.m))
			}
		}
		stroke := func(draw func(s *shape, w float64)) {
			if c, ok := paint(f.stroke, col); ok && f.strokeWidth > 0 {
				s := shape{col: c, clip: f.clip}
				draw(&s, f.strokeWidth)
				shapes = append(shapes, transformed(s, f.m))
			}
		}

		switch el.Name.Local {
		case "svg":
			// the root sets the size, which drawSVG takes from the ink instead
			if len(stack) == 1 {
				break
			}

			x, y, w, h := number(attr["x"]), number(attr["y"]), number(attr["width"]), number(attr["height"])
			a, b := f.m.apply(point{x, y}), f.m.apply(point{x + w, y + h})
			f.clip = f.clip.intersect(box{point{min(a.x, b.x), min(a.y, b.y)}, point{max(a.x, b.x), max(a.y, b.y)}})
			if vb := strings.Fields(attr["viewBox"]); len(vb) == 4 {
				f.m = f.m.mul(affine{w / number(vb[2]), 0, 0, h / number(vb[3]), x - number(vb[0])*w/number(vb[2]), y - number(vb[1])*h/number(vb[3])})
			}
		case "g":
		case "path":
			var perr error
			fill(func(s *shape) { perr = s.path(attr["d"]) })
			if perr != nil {
				return nil, nil, perr
			}
		case "polygon":
			var pts []point
			nums := pathTokenPattern.FindAllString(attr["points"], -1)
			for i := 0; i+1 < len(nums); i += 2 {
				pts = append(pts, point{number(nums[i]), number(nums[i+1])})
			}
			fill(func(s *shape) { s.polygon(pts...) })
		case "rect":
			x, y, w, h := number(attr["x"]), number(attr["y"]), number(attr["width"]), number(attr["height"])
			fill(func(s *shape) { s.polygon(point{x, y}, point{x + w, y}, point{x + w, y + h}, point{x, y + h}) })
			stroke(func(s *shape, sw float64) {
				d := sw / 2
				s.polygon(point{x - d, y - d}, point{x + w + d, y - d}, point{x + w + d, y + h + d}, point{x - d, y + h + d})
				// the inner outline runs the other way, which leaves a hole
				if w > sw && h > sw {
					s.polygon(point{x + d, y + d}, point{x + d, y + h - d}, point{x + w - d, y + h - d}, point{x + w - d, y + d})
				}
			})
		case "line":
			a, b := point{number(attr["x1"]), number(attr["y1"])}, point{number(attr["x2"]), number(attr["y2"])}
			stroke(func(s *shape, sw float64) {
				if n := math.Hypot(b.x-a.x, b.y-a.y); n > 0 {
					dx, dy := -(b.y-a.y)/n*sw/2, (b.x-a.x)/n*sw/2
					s.polygon(point{a.x + dx, a.y + dy}, point{b.x + dx, b.y + dy}, point{b.x - dx, b.y - dy}, point{a.x - dx, a.y - dy})
				}
			})
		case "text":
			var body struct {
				Text string `xml:",chardata"`
			}
			// decoding the text reads its end element too, so it doesn't push a frame
			if err := dec.DecodeElement(&body, &el); err != nil {
				return nil, nil, fmt.Errorf("failed to read math svg: %w", err)
			}

			family := attr["font-family"]
			if family == "" {
				family = "serif"
			}
			text, err := shapeText(textKey{body.Text, family, attr["font-style"] == "italic", attr["font-weight"] == "bold"})
			if err != nil {
				return nil, nil, err
			}

			// shapeText shapes at textSize, and the font size is in user units
			size := number(strings.TrimSuffix(attr["font-size"], "px")) / textSize
			m := f.m.mul(affine{size, 0, 0, size, number(attr["x"]), number(attr["y"])})
			c, filled := paint(f.fill, col)
			err = text.glyphs(func(o font.GlyphOutline, at affine) {
				if filled {
					s := shape{col: c, clip: f.clip}
					s.outline(o, m.mul(at))
					shapes = append(shapes, s)
				}
			}, func(img image.Image, at affine) {
				a, b := m.mul(at).apply(point{0, 0}), m.mul(at).apply(point{1, 1})
				pictures = append(pictures, picture{img, box{point{min(a.x, b.x), min(a.y, b.y)}, point{max(a.x, b.x), max(a.y, b.y)}}})
			})
			if err != nil {
				return nil, nil, err
			}

			continue
		default:
			return nil, nil, fmt.Errorf("math svg has unsupported element %s", el.Name.Local)
		}

		stack = append(stack, f)
	}
}

// transformed returns s with its points mapped by m.
func transformed(s shape, m affine) shape {
	for i, p := range s.pts {
		s.pts[i] = m.apply(p)
	}

	return s
}

// path adds the outline of SVG path data d, except arcs, which MathJax doesn't write.
func (s *shape) path(d string) error {
	toks := pathTokenPattern.FindAllString(d, -1)
	var cur, start, ctrl point
	var cmd, prev byte
	for i := 0; i < len(toks); {
		if c := toks[i][0]; c >= 'A' {
			cmd = c
			i++
		} else if cmd == 0 {
			return fmt.Errorf("math svg path %q starts without a command", d)
		}

		lower := cmd | 0x20
		if lower == 'z' {
			s.add('Z')
			cur, prev = start, cmd
			continue
		}

		need := map[byte]int{'m': 2, 'l': 2, 'h': 1, 'v': 1, 'q': 4, 't': 2, 'c': 6, 's': 4}[lower]
		if need == 0 || i+need > len(toks) {
			return fmt.Errorf("math svg path %q has unsupported command %c", d, cmd)
		}

		var args []float64
		for _, t := range toks[i : i+need] {
			if t[0] >= 'A' {
				return fmt.Errorf("math svg path %q has too few numbers for %c", d, cmd)
			}
			args = append(args, number(t))
		}
		i += need

		// relative commands count from the current point
		base := point{}
		if cmd == lower {
			base = cur
		}
		at := func(j int) point { return point{base.x + args[j], base.y + args[j+1]} }
		// t and s mirror the previous control point when they follow a curve of their kind
		mirror := func(kinds string) point {
			if strings.IndexByte(kinds, prev|0x20) >= 0 {
				return point{2*cur.x - ctrl.x, 2*cur.y - ctrl.y}
			}
			return cur
		}

		var next point
		switch lower {
		case 'm':
			next = at(0)
			start = next
			s.add('M', next)
			// numbers after a move are lines
			if cmd == 'm' {
				cmd = 'l'
			} else {
				cmd = 'L'
			}
		case 'l':
			next = at(0)
			s.add('L', next)
		case 'h':
			next = point{base.x + args[0], cur.y}
			s.add('L', next)
		case 'v':
			next = point{cur.x, base.y + args[0]}
			s.add('L', next)
		case 'q':
			ctrl, next = at(0), at(2)
			s.add('Q', ctrl, next)
		case 't':
			ctrl, next = mirror("qt"), at(0)
			s.add('Q', ctrl, next)
		case 'c':
			c1 := at(0)
			ctrl, next = at(2), at(4)
			s.add('C', c1, ctrl, next)
		case 's':
			c1 := mirror("cs")
			ctrl, next = at(0), at(2)
			s.add('C', c1, ctrl, next)
		}
		cur, prev = next, lower
	}

	return nil
}

// transform returns the matrix of an SVG transform attribute.
func transform(v string) (affine, error) {
	m := identity
	for _, t := range transformPattern.FindAllStringSubmatch(v, -1) {
		var a []float64
		for _, s := range pathTokenPattern.FindAllString(t[2], -1) {
			a = append(a, number(s))
		}

		switch {
		case t[1] == "translate" && len(a) == 1:
			m = m.mul(affine{1, 0, 0, 1, a[0], 0})
		case t[1] == "translate" && len(a) == 2:
			m = m.mul(affine{1, 0, 0, 1, a[0], a[1]})
		case t[1] == "scale" && len(a) == 1:
			m = m.mul(affine{a[0], 0, 0, a[0], 0, 0})
		case t[1] == "scale" && len(a) == 2:
			m = m.mul(affine{a[0], 0, 0, a[1], 0, 0})
		case t[1] == "matrix" && len(a) == 6:
			m = m.mul(affine(a))
		default:
			return m, fmt.Errorf("math svg has unsupported transform %s", t[0])
		}
	}

	return m, nil
}

// paint returns the color of an SVG fill or stroke, with col for currentColor and colors it can't read. It's false
// for none.
func paint(v string, col color.RGBA) (color.RGBA, bool) {
	if v == "none" {
		return color.RGBA{}, false
	}

	if hex, ok := strings.CutPrefix(v, "#"); ok {
		if len(hex) == 3 {
			hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
		}
		if n, err := strconv.ParseUint(hex, 16, 32); err == nil && len(hex) == 6 {
			return color.RGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 0xFF}, true
		}
	}

	if c, ok := colornames.Map[strings.ToLower(v)]; ok {
		return c, true
	}

	return col, true
}

func number(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
