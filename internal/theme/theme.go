package theme

import (
	"math/rand"

	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/sprite"
)

type Theme struct {
	Name    string
	Palette sprite.Palette
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

var themes = map[string]Theme{"soccer": Soccer}

func Get(name string) (Theme, bool) {
	t, ok := themes[name]
	return t, ok
}
