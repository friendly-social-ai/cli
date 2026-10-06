package ui

import (
	"fmt"
	"image"
	"strings"
)

// RenderImage draws img with half-block characters, each cell holding two vertically stacked pixels.
// Result fits into width cells and maxRows lines keeping aspect ratio. Requires truecolor terminal.
func RenderImage(img image.Image, width, maxRows int) string {
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 || width <= 0 || maxRows <= 0 {
		return ""
	}

	w := min(width, bounds.Dx())
	h := bounds.Dy() * w / bounds.Dx()
	if h > maxRows*2 {
		h = maxRows * 2
		w = max(bounds.Dx()*h/bounds.Dy(), 1)
	}
	h = max(h+h%2, 2)

	var b strings.Builder
	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x++ {
			tr, tg, tb, _ := average(img, bounds, x, y, w, h)
			br, bg, bb, _ := average(img, bounds, x, y+1, w, h)
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", tr, tg, tb, br, bg, bb)
		}

		b.WriteString("\x1b[0m")
		if y+2 < h {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

// average returns mean alpha-premultiplied color of source pixels covered by cell (x, y) of w*h grid.
// Ignoring alpha of the result composites it over black.
func average(img image.Image, bounds image.Rectangle, x, y, w, h int) (uint8, uint8, uint8, uint8) {
	x0 := bounds.Min.X + x*bounds.Dx()/w
	x1 := max(bounds.Min.X+(x+1)*bounds.Dx()/w, x0+1)
	y0 := bounds.Min.Y + y*bounds.Dy()/h
	y1 := max(bounds.Min.Y+(y+1)*bounds.Dy()/h, y0+1)

	var r, g, b, a, n uint64
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			pr, pg, pb, pa := img.At(px, py).RGBA()
			r, g, b, a, n = r+uint64(pr), g+uint64(pg), b+uint64(pb), a+uint64(pa), n+1
		}
	}

	return uint8(r / n >> 8), uint8(g / n >> 8), uint8(b / n >> 8), uint8(a / n >> 8)
}
