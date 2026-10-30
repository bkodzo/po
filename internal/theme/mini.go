package theme

import (
	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/sprite"
)

const (
	// MiniW is the width in terminal columns of the corner creature, which is
	// two terminal rows tall.
	MiniW = 8
	miniH = 4
	// MiniFps is the animation rate the mini frames are designed for.
	MiniFps = 4
)

// MiniFrame draws the small corner version of the creature: 8 by 4 pixels, so
// two terminal rows. The mood shows mostly in colour and movement.
func (t Theme) MiniFrame(m mood.Mood, tick int) (*sprite.Canvas, sprite.Palette) {
	c := sprite.NewCanvas(MiniW, miniH)
	dx, dy := 0, 1
	eyes, mouth := 'k', "flat"

	switch m {
	case mood.Celebrating:
		dy = 1 - tick%2
		mouth = "wide"
	case mood.Frustrated:
		if tick%12 < 8 {
			dx = [4]int{0, -1, 0, 1}[tick%4]
		}
		mouth = "frown"
	case mood.Tired:
		eyes = 's'
		mouth = "yawn"
		if tick%16 >= 4 {
			mouth = "dot"
		}
	case mood.Focused:
		mouth = "dot"
	default:
		mouth = "smile"
		if tick%20 == 19 {
			eyes = 's'
		}
	}

	row := func(y, x0, x1 int, ch rune) {
		for x := x0; x <= x1; x++ {
			c.Set(x+dx, y+dy, ch)
		}
	}
	row(0, 2, 5, 'b')
	row(1, 1, 6, 'b')
	row(2, 1, 6, 'b')
	if m == mood.Frustrated {
		c.Set(2+dx, dy, 'o')
		c.Set(5+dx, dy, 'o')
	} else {
		c.Set(2+dx, dy, 'l')
	}
	c.Set(2+dx, 1+dy, eyes)
	c.Set(5+dx, 1+dy, eyes)

	switch mouth {
	case "smile", "frown", "flat":
		row(2, 3, 4, 'm')
	case "wide":
		row(2, 2, 5, 'm')
	case "dot":
		c.Set(3+dx, 2+dy, 'm')
	case "yawn":
		row(1, 3, 4, 'm')
		row(2, 3, 4, 'm')
	}
	if m == mood.Frustrated || m == mood.Calm || m == mood.Celebrating {
		c.Set(1+dx, 2+dy, 'c')
		c.Set(6+dx, 2+dy, 'c')
	}

	// Extras live in the free top row, so they only show while resting.
	if dy == 1 {
		switch m {
		case mood.Frustrated:
			c.Set(3+tick%2, 0, 'g')
		case mood.Tired:
			c.Set(6+tick/2%2, 0, 'z')
		case mood.Celebrating:
			c.Set(0, 0, 'y')
			c.Set(7, 0, 'y')
		}
	}
	return c, t.paletteFor(m)
}
