// Package idle decides when the creature does something while nothing is
// happening: glances around, stretches, falls asleep. It knows nothing about
// drawing, only about time and mood, so it can be tested without waiting.
package idle

import (
	"math/rand"
	"time"

	"github.com/bkodzo/po/internal/mood"
)

// Config is how lively the creature is when idle.
type Config struct {
	// On switches idle animations on or off.
	On bool
	// SleepAfter is how long with no commands before it falls asleep.
	SleepAfter time.Duration
	// MinGap and MaxGap bound the quiet time between two fidgets.
	MinGap, MaxGap time.Duration
}

func DefaultConfig() Config {
	return Config{On: true, SleepAfter: 10 * time.Minute, MinGap: 25 * time.Second, MaxGap: 70 * time.Second}
}

// Scheduler picks idle activities.
type Scheduler struct {
	cfg    Config
	rng    *rand.Rand
	nextAt time.Time
}

// New returns a scheduler. The first fidget comes a little after start-up.
func New(cfg Config, rng *rand.Rand, now time.Time) *Scheduler {
	s := &Scheduler{cfg: cfg, rng: rng}
	s.nextAt = now.Add(s.gap(cfg.MinGap/2, cfg.MinGap))
	return s
}

func (s *Scheduler) SetConfig(cfg Config) { s.cfg = cfg }

func (s *Scheduler) gap(lo, hi time.Duration) time.Duration {
	if hi <= lo {
		return lo
	}
	return lo + time.Duration(s.rng.Int63n(int64(hi-lo)))
}

// weights say how often each fidget is chosen.
var weights = []struct {
	name   string
	weight int
}{{"look", 5}, {"stretch", 3}, {"peek", 2}}

// Pick says which activity to start now, or "" for none.
//
// lastEvent is when the last command arrived, resting is the creature's
// longer-term mood, and busy is true while a reaction or another activity is
// showing. Nothing starts while busy, and only a calm or focused creature
// fidgets: a frustrated or celebrating one has other things on its mind.
func (s *Scheduler) Pick(now, lastEvent time.Time, resting mood.Mood, busy bool) string {
	if !s.cfg.On || busy {
		return ""
	}
	if resting == mood.Calm && now.Sub(lastEvent) >= s.cfg.SleepAfter {
		return "sleep"
	}
	if resting != mood.Calm && resting != mood.Focused {
		return ""
	}
	if now.Before(s.nextAt) {
		return ""
	}

	total := 0
	for _, w := range weights {
		total += w.weight
	}
	pick := s.rng.Intn(total)
	chosen := weights[0].name
	for _, w := range weights {
		if pick < w.weight {
			chosen = w.name
			break
		}
		pick -= w.weight
	}

	// A focused creature fidgets half as often, so it does not distract.
	lo, hi := s.cfg.MinGap, s.cfg.MaxGap
	if resting == mood.Focused {
		lo, hi = lo*2, hi*2
	}
	s.nextAt = now.Add(s.gap(lo, hi))
	return chosen
}
