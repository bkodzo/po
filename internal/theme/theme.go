package theme

import (
	"math/rand"
	"sort"

	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/sprite"
)

type Theme struct {
	Name    string
	Palette sprite.Palette
	// Body draws the creature's body at an offset. Nil means the default round
	// blob with feet. A theme can use it to give its character a different shape.
	Body func(c *sprite.Canvas, ox, oy int)
	// Tints shift the creature's body colour toward a mood colour.
	Tints  map[mood.Mood]Tint
	Lines  map[mood.Mood][]string
	Unsure []string
}

// Tint moves the body colour part of the way toward a colour. Colours far from
// the body's own hue go muddy if pushed hard, so the strength is chosen per mood.
type Tint struct {
	Color    sprite.Color
	Strength float64
}

func (t Theme) Say(m mood.Mood) string {
	return pick(t.Lines[m])
}

func (t Theme) Confused() string {
	return pick(t.Unsure)
}

func pick(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return lines[rand.Intn(len(lines))]
}

var themes = map[string]Theme{"soccer": Soccer, "ghost": Ghost, "cat": Cat}

func Get(name string) (Theme, bool) {
	t, ok := themes[name]
	return t, ok
}

// Names lists the available themes in a stable order.
func Names() []string {
	names := make([]string, 0, len(themes))
	for n := range themes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// stamp draws the theme's body.
func (t Theme) stamp(c *sprite.Canvas, ox, oy int) {
	if t.Body != nil {
		t.Body(c, ox, oy)
		return
	}
	stampBody(c, ox, oy)
}
