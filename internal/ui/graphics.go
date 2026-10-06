package ui

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

const (
	// maxGraphicsSide limits the side of a transmitted image in pixels. Upload downscales larger images first.
	maxGraphicsSide = 1280

	// maxPlaceholderCells is the number of row and column diacritics in kitty.Diacritic table.
	maxPlaceholderCells = 297
)

// Output is a terminal file whose writes are serialized, so that graphics commands never split renderer frames.
type Output struct {
	*os.File
	mu sync.Mutex
}

// NewOutput wraps terminal file into Output.
func NewOutput(f *os.File) *Output {
	return &Output{File: f}
}

func (o *Output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.File.Write(p)
}

// WriteString shadows promoted os.File.WriteString, which io.WriteString would otherwise call bypassing the lock.
func (o *Output) WriteString(s string) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.File.WriteString(s)
}

// Graphics displays images with Kitty graphics protocol Unicode placeholders. It transmits an image once and draws
// it as placeholder text, so redraws of the program and tmux keep it in place.
type Graphics struct {
	out  io.Writer
	tmux bool

	mu  sync.Mutex
	ids map[uint32]struct{}
}

// NewGraphics returns Graphics writing to out, or nil when terminal doesn't support Unicode placeholders.
func NewGraphics(out io.Writer) *Graphics {
	tmux := os.Getenv("TMUX") != ""

	term := os.Getenv("TERM_PROGRAM")
	if os.Getenv("TERM") == "xterm-kitty" {
		term = "kitty"
	}

	if tmux {
		termtype, err := exec.Command("tmux", "display", "-p", "#{client_termtype}").Output()
		if err != nil {
			return nil
		}

		passthrough, err := exec.Command("tmux", "show", "-Apv", "allow-passthrough").Output()
		if err != nil || strings.TrimSpace(string(passthrough)) == "off" {
			return nil
		}

		term = string(termtype)
	}

	term = strings.ToLower(strings.TrimSpace(term))
	if !strings.HasPrefix(term, "ghostty") && !strings.HasPrefix(term, "kitty") {
		return nil
	}

	return &Graphics{
		out:  out,
		tmux: tmux,
		ids:  make(map[uint32]struct{}),
	}
}

func (g *Graphics) write(seq string) error {
	if g.tmux {
		seq = ansi.TmuxPassthrough(seq)
	}

	_, err := io.WriteString(g.out, seq)
	return err
}

// Upload transmits img to the terminal and places it in cols x rows cells. It returns the image ID for Placeholder.
func (g *Graphics) Upload(img image.Image, cols, rows int) (uint32, error) {
	id := rand.Uint32N(1<<24-1) + 1

	opts := &kitty.Options{
		Action:           kitty.TransmitAndPut,
		Quite:            2,
		ID:               int(id),
		PlacementID:      1,
		Format:           kitty.PNG,
		VirtualPlacement: true,
		Columns:          cols,
		Rows:             rows,
		Chunk:            true,
		Transmission:     kitty.Direct,
	}
	if g.tmux {
		opts.ChunkFormatter = ansi.TmuxPassthrough
	}

	var buf bytes.Buffer
	if err := kitty.EncodeGraphics(&buf, downscale(img, maxGraphicsSide), opts); err != nil {
		return 0, fmt.Errorf("failed to encode image: %w", err)
	}

	if _, err := g.out.Write(buf.Bytes()); err != nil {
		return 0, fmt.Errorf("failed to upload image: %w", err)
	}

	g.mu.Lock()
	g.ids[id] = struct{}{}
	g.mu.Unlock()

	return id, nil
}

// Place resizes placement of uploaded image to cols x rows cells.
func (g *Graphics) Place(id uint32, cols, rows int) error {
	opts := kitty.Options{
		Action:           kitty.Put,
		Quite:            2,
		ID:               int(id),
		PlacementID:      1,
		VirtualPlacement: true,
		Columns:          cols,
		Rows:             rows,
	}

	return g.write(ansi.KittyGraphics(nil, opts.Options()...))
}

// Delete frees uploaded image in terminal.
func (g *Graphics) Delete(id uint32) error {
	g.mu.Lock()
	delete(g.ids, id)
	g.mu.Unlock()

	opts := kitty.Options{
		Action:          kitty.Delete,
		Delete:          kitty.DeleteID,
		DeleteResources: true,
		Quite:           2,
		ID:              int(id),
	}

	return g.write(ansi.KittyGraphics(nil, opts.Options()...))
}

// Close frees all uploaded images.
func (g *Graphics) Close() {
	g.mu.Lock()
	ids := make([]uint32, 0, len(g.ids))
	for id := range g.ids {
		ids = append(ids, id)
	}
	g.mu.Unlock()

	for _, id := range ids {
		_ = g.Delete(id)
	}
}

// Placeholder returns text of cols x rows placeholder cells displaying uploaded image.
func Placeholder(id uint32, cols, rows int) string {
	color := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", id>>16&0xff, id>>8&0xff, id&0xff)

	var b strings.Builder
	for row := range rows {
		b.WriteString(color)
		for col := range cols {
			b.WriteRune(kitty.Placeholder)
			b.WriteRune(kitty.Diacritic(row))
			b.WriteRune(kitty.Diacritic(col))
		}

		b.WriteString("\x1b[39m")
		if row+1 < rows {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

// Fit returns size in cells of image fitting into maxCols x maxRows, assuming cells twice as tall as wide.
func Fit(img image.Image, maxCols, maxRows int) (int, int) {
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return 0, 0
	}

	maxCols = min(maxCols, maxPlaceholderCells)
	maxRows = min(maxRows, maxPlaceholderCells)

	cols := maxCols
	rows := (bounds.Dy()*cols + bounds.Dx()) / (2 * bounds.Dx())
	if rows > maxRows {
		rows = maxRows
		cols = 2 * bounds.Dx() * rows / bounds.Dy()
	}

	return max(cols, 1), max(rows, 1)
}

// downscale shrinks img so that its larger side is at most side pixels.
func downscale(img image.Image, side int) image.Image {
	bounds := img.Bounds()
	if bounds.Dx() <= side && bounds.Dy() <= side {
		return img
	}

	w, h := side, bounds.Dy()*side/bounds.Dx()
	if bounds.Dy() > bounds.Dx() {
		w, h = bounds.Dx()*side/bounds.Dy(), side
	}

	result := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	for y := range result.Rect.Dy() {
		for x := range result.Rect.Dx() {
			r, g, b, a := average(img, bounds, x, y, result.Rect.Dx(), result.Rect.Dy())
			i := result.PixOffset(x, y)
			result.Pix[i], result.Pix[i+1], result.Pix[i+2], result.Pix[i+3] = r, g, b, a
		}
	}

	return result
}
