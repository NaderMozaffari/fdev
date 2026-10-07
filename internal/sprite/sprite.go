// Package sprite draws fdev's own icons (platforms, sections, a star) as
// pixel art in half blocks, like the app icons in package icon: no font or
// image protocol needed. A shine can sweep across them, for the menu's
// selection.
package sprite

import (
	"fmt"
	"image/color"
	"strings"
)

// The sprites, 10×8 pixels: # is the color, + a lighter shade of it, and
// anything else is transparent.
var sprites = map[string][]string{
	"android": {
		".#......#.",
		"..#....#..",
		"..######..",
		".##.##.##.",
		".########.",
		"##########",
		"##########",
		"..........",
	},
	"ios": {
		".....+....",
		"....+.....",
		"..##.###..",
		".########.",
		".#######..",
		".#######..",
		".########.",
		"..##..##..",
	},
	"web": {
		"...####...",
		"..#.##.#..",
		".#..##..#.",
		".########.",
		".#..##..#.",
		"..#.##.#..",
		"...####...",
		"..........",
	},
	"desktop": {
		"##########",
		"#++++++++#",
		"#++++++++#",
		"#++++++++#",
		"##########",
		"....##....",
		"..######..",
		"..........",
	},
	"windows": {
		".####.++++",
		".####.++++",
		".####.++++",
		"..........",
		".++++.####",
		".++++.####",
		".++++.####",
		"..........",
	},
	"run": {
		"..#.......",
		"..###.....",
		"..#####...",
		"..#######.",
		"..#####...",
		"..###.....",
		"..#.......",
		"..........",
	},
	"build": {
		"..######..",
		".#++++++#.",
		"##########",
		"#...##...#",
		"#...++...#",
		"#........#",
		"##########",
		"..........",
	},
	"tools": {
		"...#..#...",
		".########.",
		".##....##.",
		"###.++.###",
		"###.++.###",
		".##....##.",
		".########.",
		"...#..#...",
	},
	"recent": {
		"..######..",
		".#..+...#.",
		"#...+....#",
		"#...+++..#",
		"#........#",
		".#......#.",
		"..######..",
		"..........",
	},
	"logs": {
		".#######..",
		".#.....##.",
		".#.+++..#.",
		".#......#.",
		".#.++++.#.",
		".#......#.",
		".########.",
		"..........",
	},
	"star": {
		"....##....",
		"....##....",
		"##########",
		".##++++##.",
		"..######..",
		"..##..##..",
		".##....##.",
		"..........",
	},
	"bag": {
		"...####...",
		"..#....#..",
		".########.",
		".#++++++#.",
		".#++++++#.",
		".#++++++#.",
		".########.",
		"..........",
	},
	"store": {
		"##########",
		"#+#+#+#+##",
		"##########",
		".#......#.",
		".#.##...#.",
		".#.##...#.",
		".########.",
		"..........",
	},
	"phone": {
		"..######..",
		"..#++++#..",
		"..#++++#..",
		"..#++++#..",
		"..#++++#..",
		"..#++++#..",
		"..##..##..",
		"..######..",
	},
	"refresh": {
		"...####.#.",
		"..#....##.",
		".#....###.",
		".#........",
		".#......#.",
		"..#....#..",
		"...####...",
		"..........",
	},
	"auto": {
		"....#.....",
		"....#.....",
		"..#####...",
		"....#...#.",
		"....#..###",
		"........#.",
		"..#.......",
		".###......",
	},
	"option": {
		"...####...",
		"..#....#..",
		".#..++..#.",
		".#.++++.#.",
		".#..++..#.",
		"..#....#..",
		"...####...",
		"..........",
	},
	"flavor": {
		"..######..",
		".#++++++#.",
		"#++####++#",
		"#+#....#+#",
		"#+#....#+#",
		"#++####++#",
		".#++++++#.",
		"..######..",
	},
}

// Width and Height are a sprite's size in cells, at scale 1.
const (
	Width  = 10
	Height = 4
)

// For is the sprite of a platform ("Android", "iOS", ...) or a menu section
// ("Run", "Build", "Tools", "Recent", "Saved logs"); "" when there is none.
func For(name string) string {
	switch strings.ToLower(name) {
	case "android":
		return "android"
	case "ios":
		return "ios"
	case "web":
		return "web"
	case "macos", "linux":
		return "desktop"
	case "windows":
		return "windows"
	case "run":
		return "run"
	case "build":
		return "build"
	case "tools", "other":
		return "tools"
	case "recent":
		return "recent"
	case "saved logs", "logs":
		return "logs"
	case "star", "bag", "store", "phone", "refresh", "auto", "option", "play":
		if name == "play" {
			return "run"
		}
		return name
	}
	return "flavor"
}

// Render draws a sprite in c, scale times its size (Width×Height cells at
// 1). shine, from 0 to 1, sweeps a highlight across it from the top left;
// outside that range there is none.
func Render(name string, c color.Color, scale int, shine float64) []string {
	rows := sprites[name]
	if rows == nil {
		rows = sprites["flavor"]
	}
	scale = max(scale, 1)
	base := toNRGBA(c)
	light := mix(base, color.NRGBA{255, 255, 255, 255}, 0.45)
	w, h := len(rows[0])*scale, len(rows)*scale
	band := -1.0
	if shine > 0 && shine < 1 {
		band = shine * float64(w+h+6)
	}
	px := func(x, y int) (color.NRGBA, bool) {
		var p color.NRGBA
		switch rows[y/scale][x/scale] {
		case '#':
			p = base
		case '+':
			p = light
		default:
			return p, false
		}
		if band >= 0 {
			if d := float64(x+y) - band; d > -3 && d < 3 {
				p = mix(p, color.NRGBA{255, 255, 255, 255}, 0.7*(1-abs(d)/3))
			}
		}
		return p, true
	}
	out := make([]string, h/2)
	for y := 0; y < h; y += 2 {
		var b strings.Builder
		for x := 0; x < w; x++ {
			top, topOK := px(x, y)
			bottom, bottomOK := px(x, y+1)
			b.WriteString(cell(top, topOK, bottom, bottomOK))
		}
		b.WriteString("\x1b[0m")
		out[y/2] = b.String()
	}
	return out
}

func cell(top color.NRGBA, topOK bool, bottom color.NRGBA, bottomOK bool) string {
	switch {
	case !topOK && !bottomOK:
		return "\x1b[0m "
	case !topOK:
		return fmt.Sprintf("\x1b[0;38;2;%d;%d;%dm▄", bottom.R, bottom.G, bottom.B)
	case !bottomOK:
		return fmt.Sprintf("\x1b[0;38;2;%d;%d;%dm▀", top.R, top.G, top.B)
	}
	return fmt.Sprintf("\x1b[0;38;2;%d;%d;%d;48;2;%d;%d;%dm▀", top.R, top.G, top.B, bottom.R, bottom.G, bottom.B)
}

func toNRGBA(c color.Color) color.NRGBA {
	if c == nil {
		return color.NRGBA{0x8E, 0x8E, 0x93, 255}
	}
	r, g, b, _ := c.RGBA()
	return color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
}

func mix(a, b color.NRGBA, t float64) color.NRGBA {
	f := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.NRGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 255}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
