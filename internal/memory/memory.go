// Package memory is what Po remembers from one day to the next. It keeps counts
// and nothing else: never a command, a path or any text you typed. From those
// counts it notices moments worth a word, such as the first green after a rough
// patch, and it can summarise a week.
package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	// KeepDays is how long a day is remembered.
	KeepDays = 90
	// A relief is the first success after this many failures in a row.
	reliefAfter = 3
	// A failure streak this long, if it beats the best so far, is a record.
	recordFrom = 5
	// A gap this long between commands counts as having been away.
	awayFor = 20 * time.Hour
	// A day's busy mark.
	busyMark = 100
	// At most this many reliefs are announced in one day.
	maxReliefsAnnounced = 3
)

// Day is what happened on one calendar day. Only counts.
type Day struct {
	Commands      int      `json:"commands"`
	Failures      int      `json:"failures"`
	LongestStreak int      `json:"longest_streak"`
	LateNight     int      `json:"late_night"` // commands between midnight and 5am
	Pushes        int      `json:"pushes"`
	Reliefs       int      `json:"reliefs"`
	Announced     []string `json:"announced,omitempty"` // kinds already said today
}

// Lifetime is what is remembered across all days.
type Lifetime struct {
	BestStreak int       `json:"best_streak"`
	FirstSeen  string    `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
}

// Store holds the remembered days.
type Store struct {
	mu       sync.Mutex
	path     string
	Days     map[string]*Day `json:"days"`
	Lifetime Lifetime        `json:"lifetime"`
	dirty    bool
}

// Kinds of moment worth a word, most important first.
const (
	KindRecord    = "record"   // a new longest run of failures
	KindRelief    = "relief"   // the first success after a rough patch
	KindLateRun   = "late_run" // several late nights in a row
	KindComeback  = "comeback" // back after a long time away
	KindBusy      = "busy"     // the hundredth command of the day
	KindFirstPush = "first_push"
)

// Note is a moment worth a word. N is the number it is about, if any: the
// failures behind a relief, the nights in a row, the hours away.
type Note struct {
	Kind string
	N    int
}

// Observation is one finished command, reduced to what memory needs.
type Observation struct {
	Exit       int
	Hour       int
	FailStreak int  // failures in a row before this command
	Push       bool // publishes work
	// Ignored commands (a plain ls, say) count as activity but not as outcomes.
	Ignored bool
}

// DefaultPath is where the memory is kept.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "po", "memory.json")
}

// Load reads the memory at path, or starts an empty one if there is none.
func Load(path string) *Store {
	s := &Store{path: path, Days: map[string]*Day{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	var loaded Store
	if json.Unmarshal(data, &loaded) != nil || loaded.Days == nil {
		return s // an unreadable file is replaced, not trusted
	}
	s.Days, s.Lifetime = loaded.Days, loaded.Lifetime
	return s
}

func dayKey(t time.Time) string { return t.Format("2006-01-02") }

func (s *Store) day(t time.Time) *Day {
	k := dayKey(t)
	d := s.Days[k]
	if d == nil {
		d = &Day{}
		s.Days[k] = d
	}
	return d
}

func announced(d *Day, kind string) bool {
	for _, k := range d.Announced {
		if k == kind {
			return true
		}
	}
	return false
}

// Record adds one command and returns the moments it brings about, most
// important first. Each kind is announced at most once a day (reliefs up to
// three times), so the pet speaks rarely enough for it to mean something.
func (s *Store) Record(now time.Time, o Observation) []Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty = true

	d := s.day(now)
	var notes []Note
	say := func(kind string, n int) {
		if !announced(d, kind) {
			d.Announced = append(d.Announced, kind)
			notes = append(notes, Note{kind, n})
		}
	}

	// Coming back after a long time away.
	if !s.Lifetime.LastSeen.IsZero() && now.Sub(s.Lifetime.LastSeen) >= awayFor {
		say(KindComeback, int(now.Sub(s.Lifetime.LastSeen).Hours()))
	}
	if s.Lifetime.FirstSeen == "" {
		s.Lifetime.FirstSeen = dayKey(now)
	}
	s.Lifetime.LastSeen = now

	d.Commands++
	if d.Commands == busyMark {
		say(KindBusy, busyMark)
	}
	if o.Hour >= 0 && o.Hour < 5 {
		d.LateNight++
		if d.LateNight == 1 {
			if run := s.lateNightRun(now); run >= 3 {
				say(KindLateRun, run)
			}
		}
	}
	if o.Ignored {
		return order(notes)
	}

	switch {
	case o.Exit == 0:
		if o.Push {
			d.Pushes++
			if d.Pushes == 1 {
				say(KindFirstPush, 0)
			}
		}
		if o.FailStreak >= reliefAfter {
			d.Reliefs++
			if d.Reliefs <= maxReliefsAnnounced {
				notes = append(notes, Note{KindRelief, o.FailStreak})
			}
		}
	case o.Exit < 128: // a Ctrl-C or kill is not a failure of the work
		d.Failures++
		streak := o.FailStreak + 1
		if streak > d.LongestStreak {
			d.LongestStreak = streak
		}
		if streak >= recordFrom && streak > s.Lifetime.BestStreak {
			notes = append(notes, Note{KindRecord, streak})
		}
		if streak > s.Lifetime.BestStreak {
			s.Lifetime.BestStreak = streak
		}
	}
	s.prune(now)
	return order(notes)
}

// lateNightRun counts the nights in a row, ending today, with late-night work.
func (s *Store) lateNightRun(now time.Time) int {
	run := 0
	for t := now; ; t = t.AddDate(0, 0, -1) {
		d := s.Days[dayKey(t)]
		if d == nil || d.LateNight == 0 {
			return run
		}
		run++
	}
}

var priority = map[string]int{
	KindRecord: 0, KindRelief: 1, KindLateRun: 2, KindComeback: 3, KindBusy: 4, KindFirstPush: 5,
}

func order(notes []Note) []Note {
	sort.SliceStable(notes, func(i, j int) bool { return priority[notes[i].Kind] < priority[notes[j].Kind] })
	return notes
}

// prune forgets days older than KeepDays.
func (s *Store) prune(now time.Time) {
	cutoff := dayKey(now.AddDate(0, 0, -KeepDays))
	for k := range s.Days {
		if k < cutoff {
			delete(s.Days, k)
		}
	}
}

// Dirty reports whether there is anything not yet saved.
func (s *Store) Dirty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirty
}

// Save writes the memory out, replacing the file in one step.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return nil
	}
	data, err := json.Marshal(struct {
		Days     map[string]*Day `json:"days"`
		Lifetime Lifetime        `json:"lifetime"`
	}{s.Days, s.Lifetime})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	s.dirty = false
	return nil
}

// Week is a summary of the last few days.
type Week struct {
	Days          int // how many days were looked at
	ActiveDays    int
	Commands      int
	Failures      int
	LongestStreak int
	LongestOn     string // the weekday of the longest streak
	LateNights    int    // days with late-night work
	Reliefs       int
	Pushes        int
	BusiestDay    string // the weekday with the most commands
	BusiestCount  int
}

// Summary adds up the last n days, ending today.
func (s *Store) Summary(now time.Time, n int) Week {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := Week{Days: n}
	for i := 0; i < n; i++ {
		t := now.AddDate(0, 0, -i)
		d := s.Days[dayKey(t)]
		if d == nil || d.Commands == 0 {
			continue
		}
		w.ActiveDays++
		w.Commands += d.Commands
		w.Failures += d.Failures
		w.Reliefs += d.Reliefs
		w.Pushes += d.Pushes
		if d.LateNight > 0 {
			w.LateNights++
		}
		if d.LongestStreak > w.LongestStreak {
			w.LongestStreak, w.LongestOn = d.LongestStreak, t.Weekday().String()
		}
		if d.Commands > w.BusiestCount {
			w.BusiestCount, w.BusiestDay = d.Commands, t.Weekday().String()
		}
	}
	return w
}
