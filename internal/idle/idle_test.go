package idle

import (
	"math/rand"
	"testing"
	"time"

	"github.com/bkodzo/po/internal/mood"
)

var start = time.Date(2026, 10, 31, 14, 0, 0, 0, time.UTC)

func newScheduler() *Scheduler {
	return New(DefaultConfig(), rand.New(rand.NewSource(1)), start)
}

// run steps through time and returns what was picked and when.
func run(s *Scheduler, from time.Time, seconds int, lastEvent time.Time, resting mood.Mood) map[string]int {
	got := map[string]int{}
	for i := 0; i < seconds; i++ {
		now := from.Add(time.Duration(i) * time.Second)
		if name := s.Pick(now, lastEvent, resting, false); name != "" {
			got[name]++
		}
	}
	return got
}

func TestNothingHappensRightAfterStartUp(t *testing.T) {
	s := newScheduler()
	for i := 0; i < 10; i++ {
		if name := s.Pick(start.Add(time.Duration(i)*time.Second), start, mood.Calm, false); name != "" {
			t.Fatalf("started %q after %d s; the first fidget should wait a little", name, i)
		}
	}
}

func TestACalmCreatureFidgetsNowAndThenButNotConstantly(t *testing.T) {
	s := newScheduler()
	// Ten minutes of calm, but a command every minute so it never falls asleep.
	total := 0
	for minute := 0; minute < 10; minute++ {
		got := run(s, start.Add(time.Duration(minute)*time.Minute), 60, start.Add(time.Duration(minute)*time.Minute), mood.Calm)
		for name, n := range got {
			if name != "look" && name != "stretch" {
				t.Fatalf("unexpected activity %q", name)
			}
			total += n
		}
	}
	// With gaps of 25 to 70 seconds that is roughly 8 to 24 fidgets in ten minutes.
	if total < 6 || total > 26 {
		t.Errorf("%d fidgets in ten minutes: too few to feel alive or too many to be calm", total)
	}
}

func TestGapsRespectTheMinimum(t *testing.T) {
	s := newScheduler()
	var last time.Time
	for i := 0; i < 1200; i++ {
		now := start.Add(time.Duration(i) * time.Second)
		if name := s.Pick(now, now, mood.Calm, false); name != "" {
			if !last.IsZero() && now.Sub(last) < 25*time.Second {
				t.Fatalf("two fidgets only %v apart", now.Sub(last))
			}
			last = now
		}
	}
	if last.IsZero() {
		t.Fatal("it never fidgeted")
	}
}

func TestSleepsAfterAQuietSpellButOnlyWhenCalm(t *testing.T) {
	s := newScheduler()
	last := start
	if got := s.Pick(start.Add(9*time.Minute+59*time.Second), last, mood.Calm, false); got == "sleep" {
		t.Error("fell asleep a second too early")
	}
	if got := s.Pick(start.Add(10*time.Minute), last, mood.Calm, false); got != "sleep" {
		t.Errorf("after ten quiet minutes got %q, want sleep", got)
	}
	// A creature that is still frustrated does not nod off.
	if got := s.Pick(start.Add(30*time.Minute), last, mood.Frustrated, false); got != "" {
		t.Errorf("a frustrated creature picked %q", got)
	}
}

func TestNothingStartsWhileBusy(t *testing.T) {
	s := newScheduler()
	far := start.Add(time.Hour)
	if got := s.Pick(far, start, mood.Calm, true); got != "" {
		t.Errorf("picked %q while busy", got)
	}
}

func TestOnlyCalmOrFocusedFidget(t *testing.T) {
	for _, m := range []mood.Mood{mood.Frustrated, mood.Celebrating, mood.Tired} {
		s := newScheduler()
		got := run(s, start.Add(time.Minute), 600, start.Add(time.Minute), m)
		if len(got) != 0 {
			t.Errorf("a %s creature fidgeted: %v", m, got)
		}
	}
}

func TestFocusedFidgetsLessThanCalm(t *testing.T) {
	count := func(m mood.Mood) int {
		n := 0
		for seed := int64(1); seed <= 20; seed++ {
			s := New(DefaultConfig(), rand.New(rand.NewSource(seed)), start)
			for _, v := range run(s, start, 3000, start.Add(time.Hour*100), m) {
				n += v
			}
		}
		return n
	}
	// lastEvent far in the future keeps it from sleeping.
	calm, focused := count(mood.Calm), count(mood.Focused)
	if focused >= calm {
		t.Errorf("focused fidgeted %d times, calm %d; focused should be quieter", focused, calm)
	}
}

func TestSwitchedOffMeansSilence(t *testing.T) {
	cfg := DefaultConfig()
	cfg.On = false
	s := New(cfg, rand.New(rand.NewSource(1)), start)
	if got := run(s, start, 7200, start, mood.Calm); len(got) != 0 {
		t.Errorf("idle was off but got %v", got)
	}
	cfg.On = true
	s.SetConfig(cfg)
	if got := s.Pick(start.Add(11*time.Minute), start, mood.Calm, false); got != "sleep" {
		t.Errorf("switching it back on should work immediately, got %q", got)
	}
}

func TestBothFidgetsAreChosen(t *testing.T) {
	seen := map[string]bool{}
	for seed := int64(1); seed <= 30; seed++ {
		s := New(DefaultConfig(), rand.New(rand.NewSource(seed)), start)
		for name := range run(s, start, 1200, start.Add(100*time.Hour), mood.Calm) {
			seen[name] = true
		}
	}
	if !seen["look"] || !seen["stretch"] {
		t.Errorf("only saw %v", seen)
	}
}
