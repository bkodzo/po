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
	Version int    `json:"version,omitempty"`
	Name    string `json:"name"`
	// Available lists every theme Po has, for a window's theme menu.
	Available []string            `json:"available,omitempty"`
	Fps       map[string]int      `json:"fps,omitempty"`
	Moods     map[string]MoodPack `json:"moods,omitempty"`
	// Activities are the idle animations, played while the creature is at rest.
	Activities map[string]ActivityPack `json:"activities,omitempty"`
	Lines      map[string][]string     `json:"lines,omitempty"`
	Unsure     []string                `json:"unsure,omitempty"`
	// Milestones and Recaps are the pet's lines for notable moments and weeks.
	Milestones map[string][]string `json:"milestones,omitempty"`
	Recaps     map[string][]string `json:"recaps,omitempty"`

	// The fields below are for hand-written theme files. A reskin names a
	// built-in character as its Base and changes only its colours, tints and
	// words. A full pack leaves them out and spells out every frame in Moods.
	Base    string              `json:"base,omitempty"`
	Palette map[string][3]int   `json:"palette,omitempty"`
	Tints   map[string]TintSpec `json:"tints,omitempty"`
}

// TintSpec moves a mood's body colour part of the way toward Color.
type TintSpec struct {
	Color    [3]int  `json:"color"`
	Strength float64 `json:"strength"`
}

// MoodPack is how one mood looks: its colours and its looping animations.
type MoodPack struct {
	Palette map[string][3]int `json:"palette"`
	Full    Animation         `json:"full"`
	Mini    Animation         `json:"mini"`
}

// ActivityPack is an idle animation: its colours, its pictures, and whether it
// repeats (sleeping) or plays once.
type ActivityPack struct {
	Palette map[string][3]int `json:"palette"`
	Loop    bool              `json:"loop"`
	Animation
}

// Animation is a loop. Frames are unique pictures, each as W*H palette letters
// in row order ("." is transparent); Sequence lists which one to show at each
// tick of the loop.
type Animation struct {
	W        int         `json:"w"`
	H        int         `json:"h"`
	Frames   []FrameText `json:"frames"`
	Sequence []int       `json:"sequence"`
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
			a.Frames = append(a.Frames, FrameText(key))
		}
		a.Sequence = append(a.Sequence, i)
	}
	return a
}

// Export describes the theme as a Pack.
func (t Theme) Export() Pack {
	if t.data != nil {
		cp := *t.data
		cp.Name, cp.Version, cp.Available = t.Name, PackVersion, Names()
		if cp.Fps == nil {
			cp.Fps = map[string]int{"full": Fps, "mini": MiniFps}
		}
		if cp.Activities == nil {
			cp.Activities = map[string]ActivityPack{}
		}
		return cp
	}
	p := Pack{
		Version:    PackVersion,
		Name:       t.Name,
		Available:  Names(),
		Fps:        map[string]int{"full": Fps, "mini": MiniFps},
		Moods:      map[string]MoodPack{},
		Activities: map[string]ActivityPack{},
		Lines:      map[string][]string{},
		Unsure:     t.Unsure,
		Milestones: t.Milestones,
		Recaps:     t.Recaps,
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
	calm := map[string][3]int{}
	for k, c := range t.paletteFor(mood.Calm) {
		calm[string(k)] = [3]int{int(c.R), int(c.G), int(c.B)}
	}
	for _, name := range ActivityNames() {
		name := name
		info := activities[name]
		p.Activities[name] = ActivityPack{
			Palette: calm,
			Loop:    info.Loop,
			Animation: animation(func(tick int) *sprite.Canvas {
				c, _ := t.ActivityFrame(name, tick)
				return c
			}, info.Ticks),
		}
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
