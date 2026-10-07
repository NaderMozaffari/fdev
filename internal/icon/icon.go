// Package icon draws a PNG (an app icon) as terminal text: each cell is two
// pixels, the upper one the foreground of "▀" and the lower one its
// background, so it works in any terminal with colors, no image protocol.
package icon

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // icons may be JPEG
	_ "image/png"
	"os"
	"strings"
	"sync"
)

var (
	mu    sync.Mutex
	cache = map[string][]string{}
)

// Render draws the image at path in cols×rows cells (cols×2rows pixels).
// It returns nil when the file can't be read.
func Render(path string, cols, rows int) []string {
	key := fmt.Sprintf("%s@%dx%d", path, cols, rows)
	mu.Lock()
	defer mu.Unlock()
	if lines, ok := cache[key]; ok {
		return lines
	}
	lines := render(path, cols, rows)
	cache[key] = lines
	return lines
}

func render(path string, cols, rows int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	px := scale(img, cols, rows*2)
	lines := make([]string, rows)
	for y := 0; y < rows; y++ {
		var b strings.Builder
		for x := 0; x < cols; x++ {
			b.WriteString(cell(px[2*y][x], px[2*y+1][x]))
		}
		b.WriteString("\x1b[0m")
		lines[y] = b.String()
	}
	return lines
}

// cell is one character cell with its upper and lower pixel.
func cell(top, bottom color.NRGBA) string {
	switch {
	case top.A < 128 && bottom.A < 128:
		return "\x1b[0m "
	case top.A < 128:
		return fmt.Sprintf("\x1b[0;38;2;%d;%d;%dm▄", bottom.R, bottom.G, bottom.B)
	case bottom.A < 128:
		return fmt.Sprintf("\x1b[0;38;2;%d;%d;%dm▀", top.R, top.G, top.B)
	}
	return fmt.Sprintf("\x1b[0;38;2;%d;%d;%d;48;2;%d;%d;%dm▀", top.R, top.G, top.B, bottom.R, bottom.G, bottom.B)
}

// scale averages the image into w×h pixels (a box filter, weighted by
// alpha so transparent corners don't darken the edges).
func scale(img image.Image, w, h int) [][]color.NRGBA {
	b := img.Bounds()
	out := make([][]color.NRGBA, h)
	for y := 0; y < h; y++ {
		out[y] = make([]color.NRGBA, w)
		y0 := b.Min.Y + y*b.Dy()/h
		y1 := max(b.Min.Y+(y+1)*b.Dy()/h, y0+1)
		for x := 0; x < w; x++ {
			x0 := b.Min.X + x*b.Dx()/w
			x1 := max(b.Min.X+(x+1)*b.Dx()/w, x0+1)
			var r, g, bl, a, n uint64
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					c := color.NRGBAModel.Convert(img.At(xx, yy)).(color.NRGBA)
					r += uint64(c.R) * uint64(c.A)
					g += uint64(c.G) * uint64(c.A)
					bl += uint64(c.B) * uint64(c.A)
					a += uint64(c.A)
					n++
				}
			}
			if a == 0 {
				continue
			}
			out[y][x] = color.NRGBA{R: uint8(r / a), G: uint8(g / a), B: uint8(bl / a), A: uint8(a / n)}
		}
	}
	return out
}
