package theme

import (
	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/sprite"
)

func palFrom(m map[string][3]int) sprite.Palette {
	pal := make(sprite.Palette, len(m))
	for k, v := range m {
		if r := []rune(k); len(r) == 1 {
			pal[r[0]] = pickColour(v)
		}
	}
	return pal
}

func canvasFrom(w, h int, letters string) *sprite.Canvas {
	c := sprite.NewCanvas(w, h)
	for i, r := range letters {
		c.Set(i%w, i/w, r)
	}
	return c
}

// dataFrame plays a picture from the theme file.
func (t Theme) dataFrame(m mood.Mood, tick int, mini bool) (*sprite.Canvas, sprite.Palette) {
	mp, ok := t.data.Moods[string(m)]
	if !ok {
		mp = t.data.Moods["calm"]
	}
	a := mp.Full
	if mini {
		a = mp.Mini
	}
	if len(a.Sequence) == 0 || len(a.Frames) == 0 {
		return sprite.NewCanvas(a.W, a.H), palFrom(mp.Palette)
	}
	idx := a.Sequence[tick%len(a.Sequence)]
	return canvasFrom(a.W, a.H, string(a.Frames[idx])), palFrom(mp.Palette)
}

// dataActivity plays an idle animation from the theme file, or the calm face if
// the file does not have that one.
func (t Theme) dataActivity(name string, tick int) (*sprite.Canvas, sprite.Palette) {
	a, ok := t.data.Activities[name]
	if !ok || len(a.Sequence) == 0 {
		return t.dataFrame(mood.Calm, tick, false)
	}
	i := tick
	if a.Loop {
		i %= len(a.Sequence)
	} else if i >= len(a.Sequence) {
		i = len(a.Sequence) - 1 // a one-off holds its last picture
	}
	return canvasFrom(a.W, a.H, string(a.Frames[a.Sequence[i]])), palFrom(a.Palette)
}
