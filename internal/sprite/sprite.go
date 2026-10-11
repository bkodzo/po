package sprite

import (
	"fmt"
	"strings"
)

type Color struct{ R, G, B uint8 }

// Mix blends c toward o by t, where 0 keeps c and 1 gives o.
func (c Color) Mix(o Color, t float64) Color {
	mix := func(a, b uint8) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }
	return Color{mix(c.R, o.R), mix(c.G, o.G), mix(c.B, o.B)}
}

type Palette map[rune]Color

const Clear = '.'

// Canvas is a grid of palette keys. Each terminal row shows two pixel rows.
type Canvas struct {
	W, H int
	px   [][]rune
}

func NewCanvas(w, h int) *Canvas {
	if h%2 != 0 {
		h++
	}
	px := make([][]rune, h)
	for y := range px {
		px[y] = []rune(strings.Repeat(string(Clear), w))
	}
	return &Canvas{W: w, H: h, px: px}
}

func (c *Canvas) Set(x, y int, r rune) {
	if x >= 0 && x < c.W && y >= 0 && y < c.H {
		c.px[y][x] = r
	}
}

func (c *Canvas) Get(x, y int) rune {
	if x < 0 || x >= c.W || y < 0 || y >= c.H {
		return Clear
	}
	return c.px[y][x]
}

// Letters returns the grid as plain text, for checking a design by eye.
func (c *Canvas) Letters() string {
	rows := make([]string, c.H)
	for y, row := range c.px {
		rows[y] = string(row)
	}
	return strings.Join(rows, "\n")
}

func (c *Canvas) Render(p Palette) string {
	var b strings.Builder
	for y := 0; y < c.H; y += 2 {
		for x := 0; x < c.W; x++ {
			top, topOK := p[c.px[y][x]]
			bot, botOK := p[c.px[y+1][x]]
			switch {
			case topOK && botOK:
				fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀\x1b[0m", top.R, top.G, top.B, bot.R, bot.G, bot.B)
			case topOK:
				fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm▀\x1b[0m", top.R, top.G, top.B)
			case botOK:
				fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm▄\x1b[0m", bot.R, bot.G, bot.B)
			default:
				b.WriteByte(' ')
			}
		}
		if y+2 < c.H {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func RGB(r, g, b uint8) Color { return Color{r, g, b} }
