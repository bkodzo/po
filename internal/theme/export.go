package theme

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/sprite"
)

// PackVersion is bumped whenever the exported format changes in a way a player
// would notice.
const PackVersion = 1

// Pack is a theme as plain data, so a program in another language can play it
// without any drawing code of its own.
type Pack struct {
	Version int                 `json:"version"`
	Name    string              `json:"name"`
	Fps     map[string]int      `json:"fps"`
	Moods   map[string]MoodPack `json:"moods"`
	Lines   map[string][]string `json:"lines"`
	Unsure  []string            `json:"unsure"`
}

// MoodPack is how one mood looks: its colours and its looping animations.
type MoodPack struct {
	Palette map[string][3]int `json:"palette"`
	Full    Animation         `json:"full"`
	Mini    Animation         `json:"mini"`
}

// Animation is a loop. Frames are unique pictures, each as W*H palette letters
// in row order ("." is transparent); Sequence lists which one to show at each
// tick of the loop.
type Animation struct {
	W        int      `json:"w"`
	H        int      `json:"h"`
	Frames   []string `json:"frames"`
	Sequence []int    `json:"sequence"`
}

// Loop lengths in ticks, chosen so every moving part of a mood comes back to
// where it started. A test checks that each loop really repeats.
var fullPeriod = map[mood.Mood]int{
	mood.Calm: 80, mood.Celebrating: 8, mood.Frustrated: 120, mood.Tired: 32, mood.Focused: 168,
}

var miniPeriod = map[mood.Mood]int{
	mood.Calm: 20, mood.Celebrating: 2, mood.Frustrated: 12, mood.Tired: 16, mood.Focused: 1,
}

func animation(frame func(tick int) *sprite.Canvas, period int) Animation {
	var a Animation
	index := map[string]int{}
	for t := 0; t < period; t++ {
		c := frame(t)
		a.W, a.H = c.W, c.H
		key := strings.ReplaceAll(c.Letters(), "\n", "")
		i, seen := index[key]
		if !seen {
			i = len(a.Frames)
			index[key] = i
			a.Frames = append(a.Frames, key)
		}
		a.Sequence = append(a.Sequence, i)
	}
	return a
}

// Export describes the theme as a Pack.
func (t Theme) Export() Pack {
	p := Pack{
		Version: PackVersion,
		Name:    t.Name,
		Fps:     map[string]int{"full": Fps, "mini": MiniFps},
		Moods:   map[string]MoodPack{},
		Lines:   map[string][]string{},
		Unsure:  t.Unsure,
	}
	for _, m := range mood.All {
		pal := map[string][3]int{}
		for k, c := range t.paletteFor(m) {
			pal[string(k)] = [3]int{int(c.R), int(c.G), int(c.B)}
		}
		m := m
		p.Moods[string(m)] = MoodPack{
			Palette: pal,
			Full: animation(func(tick int) *sprite.Canvas {
				c, _ := t.Frame(m, tick)
				return c
			}, fullPeriod[m]),
			Mini: animation(func(tick int) *sprite.Canvas {
				c, _ := t.MiniFrame(m, tick)
				return c
			}, miniPeriod[m]),
		}
		p.Lines[string(m)] = t.Lines[m]
	}
	return p
}

// WriteFile writes the exported theme as JSON, replacing any earlier file in one
// step so a reader never sees half of it.
func (t Theme) WriteFile(path string) error {
	data, err := json.Marshal(t.Export())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
