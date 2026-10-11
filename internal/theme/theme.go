package theme

import (
	"math/rand"

	"github.com/bkodzo/critter/internal/mood"
)

type Theme struct {
	Name   string
	Art    map[mood.Mood]string
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
