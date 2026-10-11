package theme

import (
	"math/rand"

	"github.com/bkodzo/critter/internal/mood"
	"github.com/bkodzo/critter/internal/sprite"
)

type Theme struct {
	Name    string
	Palette sprite.Palette
	// Tints shift the creature's body colour toward a mood colour.
	Tints  map[mood.Mood]sprite.Color
	Lines  map[mood.Mood][]string
	Unsure []string
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
