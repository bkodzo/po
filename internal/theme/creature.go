package theme

import (
	"github.com/bkodzo/critter/internal/mood"
	"github.com/bkodzo/critter/internal/sprite"
)

const (
	CanvasW = 20
	CanvasH = 24
	bodyX   = 2
	bodyY   = 6
	groundY = CanvasH - 1
	// Ticks per second the animation is designed for.
	Fps = 8
)

// Half the width of each body row, from the top of the head down.
var halfWidths = [14]int{4, 5, 6, 7, 7, 7, 7, 7, 7, 7, 7, 6, 6, 5}

type pose struct {
	dx, dy int
	eyes   string
	gaze   int
	brows  string
	mouth  string
	arms   string
	cheeks bool
}

// Frame draws the creature for a mood at a point in its animation.
func (t Theme) Frame(m mood.Mood, tick int) (*sprite.Canvas, sprite.Palette) {
	p := poseFor(m, tick)
	c := sprite.NewCanvas(CanvasW, CanvasH)

	shadow(c, p.dy)
	stampBody(c, bodyX+p.dx, bodyY+p.dy)
	face(c, bodyX+p.dx, bodyY+p.dy, p)
	extras(c, m, tick)
	return c, t.paletteFor(m)
}

func (t Theme) paletteFor(m mood.Mood) sprite.Palette {
	pal := make(sprite.Palette, len(t.Palette))
	for k, v := range t.Palette {
		pal[k] = v
	}
	if tint, ok := t.Tints[m]; ok {
		for _, k := range "blsf" {
			pal[k] = pal[k].Mix(tint, 0.45)
		}
	}
	return pal
}

func poseFor(m mood.Mood, t int) pose {
	switch m {
	case mood.Celebrating:
		jump := [8]int{0, -1, -3, -4, -3, -1, 0, 0}
		return pose{dy: jump[t%8], eyes: "happy", mouth: "grin", arms: "up", cheeks: true}
	case mood.Frustrated:
		p := pose{eyes: "angry", brows: "angry", mouth: "frown", arms: "down", cheeks: true}
		if t%24 < 12 {
			p.dx = [4]int{0, -1, 0, 1}[t%4]
		}
		return p
	case mood.Tired:
		p := pose{eyes: "closed", mouth: "flat", arms: "droop", dy: [4]int{0, 0, 1, 1}[(t/4)%4]}
		if t%32 < 8 {
			p.mouth = "yawn"
		}
		return p
	case mood.Focused:
		p := pose{eyes: "open", brows: "flat", mouth: "flat", arms: "down"}
		p.gaze = [4]int{-1, 0, 1, 0}[(t/6)%4]
		if t%56 == 55 {
			p.eyes = "blink"
		}
		return p
	default:
		p := pose{eyes: "open", mouth: "smile", arms: "down", cheeks: true}
		if t%16 >= 8 {
			p.dy = 1
		}
		if t%40 == 37 || t%40 == 38 {
			p.eyes = "blink"
		}
		return p
	}
}

func shadow(c *sprite.Canvas, dy int) {
	w := 12 + dy*2
	if w < 4 {
		w = 4
	}
	start := bodyX + 8 - w/2
	for x := start; x < start+w; x++ {
		c.Set(x, groundY, 'h')
	}
}

func stampBody(c *sprite.Canvas, ox, oy int) {
	last := len(halfWidths) - 1
	for y, hw := range halfWidths {
		l, r := 8-hw, 8+hw-1
		for x := l; x <= r; x++ {
			ch := 'b'
			switch {
			case y == 0 || y == last || x == l || x == r:
				ch = 'o'
			case y == 12 || (y == 11 && (x == l+1 || x == r-1)):
				ch = 's'
			case (y == 1 && (x == 5 || x == 6)) || (y == 2 && (x == 4 || x == 5)) || (y == 3 && x == 3):
				ch = 'l'
			}
			c.Set(ox+x, oy+y, ch)
		}
	}
	for _, x0 := range []int{4, 9} {
		for x := x0; x < x0+3; x++ {
			c.Set(ox+x, oy+14, 'f')
			c.Set(ox+x, oy+15, 'o')
		}
	}
}

func plot(c *sprite.Canvas, ox, oy int, ch rune, pts ...[2]int) {
	for _, p := range pts {
		c.Set(ox+p[0], oy+p[1], ch)
	}
}

func face(c *sprite.Canvas, ox, oy int, p pose) {
	g := p.gaze
	switch p.eyes {
	case "open":
		for _, x := range []int{4, 10} {
			plot(c, ox, oy, 'k', [2]int{x + g, 5}, [2]int{x + 1 + g, 5}, [2]int{x + g, 6}, [2]int{x + 1 + g, 6})
			plot(c, ox, oy, 'w', [2]int{x + g, 5})
		}
	case "blink", "closed":
		plot(c, ox, oy, 'k', [2]int{4, 6}, [2]int{5, 6}, [2]int{10, 6}, [2]int{11, 6})
	case "happy":
		plot(c, ox, oy, 'k', [2]int{3, 6}, [2]int{4, 5}, [2]int{5, 5}, [2]int{6, 6})
		plot(c, ox, oy, 'k', [2]int{9, 6}, [2]int{10, 5}, [2]int{11, 5}, [2]int{12, 6})
	case "angry":
		plot(c, ox, oy, 'k', [2]int{4, 6}, [2]int{5, 6}, [2]int{10, 6}, [2]int{11, 6})
	}

	switch p.brows {
	case "angry":
		plot(c, ox, oy, 'o', [2]int{3, 4}, [2]int{4, 4}, [2]int{5, 5}, [2]int{6, 5})
		plot(c, ox, oy, 'o', [2]int{12, 4}, [2]int{11, 4}, [2]int{10, 5}, [2]int{9, 5})
	case "flat":
		plot(c, ox, oy, 'o', [2]int{3 + g, 4}, [2]int{4 + g, 4}, [2]int{5 + g, 4})
		plot(c, ox, oy, 'o', [2]int{10 + g, 4}, [2]int{11 + g, 4}, [2]int{12 + g, 4})
	}

	switch p.mouth {
	case "smile":
		plot(c, ox, oy, 'm', [2]int{6, 9}, [2]int{7, 10}, [2]int{8, 10}, [2]int{9, 9})
	case "grin":
		plot(c, ox, oy, 'm', [2]int{6, 9}, [2]int{9, 9}, [2]int{7, 10}, [2]int{8, 10})
		plot(c, ox, oy, 'w', [2]int{7, 9}, [2]int{8, 9})
	case "frown":
		plot(c, ox, oy, 'm', [2]int{6, 10}, [2]int{7, 9}, [2]int{8, 9}, [2]int{9, 10})
	case "flat":
		plot(c, ox, oy, 'm', [2]int{7, 10}, [2]int{8, 10})
	case "yawn":
		plot(c, ox, oy, 'm', [2]int{7, 9}, [2]int{8, 9}, [2]int{7, 10}, [2]int{8, 10})
	}

	if p.cheeks {
		plot(c, ox, oy, 'c', [2]int{2, 8}, [2]int{3, 8}, [2]int{12, 8}, [2]int{13, 8})
	}

	switch p.arms {
	case "down":
		plot(c, ox, oy, 'b', [2]int{0, 8}, [2]int{0, 9}, [2]int{15, 8}, [2]int{15, 9})
		plot(c, ox, oy, 'o', [2]int{0, 10}, [2]int{15, 10})
	case "up":
		plot(c, ox, oy, 'b', [2]int{0, 2}, [2]int{0, 3}, [2]int{15, 2}, [2]int{15, 3})
		plot(c, ox, oy, 'o', [2]int{0, 1}, [2]int{15, 1})
	case "droop":
		plot(c, ox, oy, 'b', [2]int{0, 10}, [2]int{1, 11}, [2]int{15, 10}, [2]int{14, 11})
		plot(c, ox, oy, 'o', [2]int{1, 12}, [2]int{14, 12})
	}
}

func extras(c *sprite.Canvas, m mood.Mood, t int) {
	cx := bodyX + 8
	switch m {
	case mood.Frustrated:
		// Steam puffs rising off the head.
		rise := func(phase int) int { return bodyY - 1 - ((t/2+phase)%5)*1 }
		for _, p := range []struct{ x, ph int }{{cx - 4, 0}, {cx + 3, 2}} {
			y := rise(p.ph)
			c.Set(p.x, y, 'g')
			c.Set(p.x+1, y, 'g')
			c.Set(p.x, y-1, 'g')
		}
	case mood.Tired:
		// A drifting Z.
		step := (t / 4) % 4
		x, y := cx+5+step%2, bodyY-2-step
		for _, px := range [][2]int{{0, 0}, {1, 0}, {2, 0}, {1, 1}, {0, 2}, {1, 2}, {2, 2}} {
			c.Set(x+px[0], y+px[1], 'z')
		}
	case mood.Celebrating:
		spots := [][2]int{{1, 8}, {17, 2}, {1, 14}, {18, 11}}
		for i, s := range spots {
			if (t/2+i)%2 == 0 {
				for _, d := range [][2]int{{0, 0}, {-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
					c.Set(s[0]+d[0], s[1]+d[1], 'y')
				}
			}
		}
	}
}
