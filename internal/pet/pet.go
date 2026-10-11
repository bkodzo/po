package pet

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/bkodzo/po/internal/mood"
)

const (
	decay            = 0.6
	DefaultThreshold = 0.45

	// halfLife is how long it takes a mood to fade to half strength when
	// nothing is happening, and floor is the strength below which the pet is
	// simply calm.
	halfLife = 90 * time.Second
	floor    = 0.15
)

type Pet struct {
	Mood     mood.Mood             `json:"mood"`
	Momentum map[mood.Mood]float64 `json:"momentum"`

	// MinConfidence is how sure a reading must be to count. Readers differ in
	// how peaked their scores are, so the caller sets it per reader.
	MinConfidence float64 `json:"-"`

	// Updated is when the state was last saved, so a mood can fade while Po
	// isn't running.
	Updated time.Time `json:"updated"`
}

func New() *Pet {
	return &Pet{Mood: mood.Calm, Momentum: map[mood.Mood]float64{}, MinConfidence: DefaultThreshold}
}

// Apply folds a reading into the pet's mood and reports whether it was
// confident enough to count. Unsure readings leave the pet untouched.
func (p *Pet) Apply(r mood.Reading) bool {
	top, conf := r.Top()
	if conf < p.MinConfidence {
		return false
	}
	for _, m := range mood.All {
		p.Momentum[m] *= decay
	}
	p.Momentum[top] += conf
	p.settle()
	return true
}

// settle picks the strongest mood, or calm if none is strong enough.
func (p *Pet) settle() {
	best, score := mood.Calm, floor
	for _, m := range mood.All {
		if p.Momentum[m] > score {
			best, score = m, p.Momentum[m]
		}
	}
	p.Mood = best
}

// Decay fades every mood as time passes, so a bad run doesn't last forever.
func (p *Pet) Decay(d time.Duration) {
	if d <= 0 {
		return
	}
	f := math.Pow(0.5, d.Seconds()/halfLife.Seconds())
	for m := range p.Momentum {
		p.Momentum[m] *= f
	}
	p.settle()
}

func statePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "po", "state.json"), nil
}

func Load() *Pet {
	p := New()
	path, err := statePath()
	if err != nil {
		return p
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return p
	}
	if json.Unmarshal(data, p) != nil || p.Momentum == nil {
		return New()
	}
	p.MinConfidence = DefaultThreshold
	if !p.Updated.IsZero() {
		p.Decay(time.Since(p.Updated))
	}
	return p
}

func (p *Pet) Save() error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	p.Updated = time.Now()
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
