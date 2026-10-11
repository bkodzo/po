package pet

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/bkodzo/po/internal/mood"
)

const (
	decay            = 0.6
	DefaultThreshold = 0.45
)

type Pet struct {
	Mood     mood.Mood             `json:"mood"`
	Momentum map[mood.Mood]float64 `json:"momentum"`

	// MinConfidence is how sure a reading must be to count. Readers differ in
	// how peaked their scores are, so the caller sets it per reader.
	MinConfidence float64 `json:"-"`
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
	best, score := p.Mood, p.Momentum[p.Mood]
	for _, m := range mood.All {
		if p.Momentum[m] > score {
			best, score = m, p.Momentum[m]
		}
	}
	p.Mood = best
	return true
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
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
