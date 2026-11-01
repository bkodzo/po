package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var noon = time.Date(2026, 11, 2, 12, 0, 0, 0, time.Local) // a Monday

func store(t *testing.T) *Store { t.Helper(); return Load(filepath.Join(t.TempDir(), "memory.json")) }

func kinds(notes []Note) []string {
	var k []string
	for _, n := range notes {
		k = append(k, n.Kind)
	}
	return k
}

func has(notes []Note, kind string) (Note, bool) {
	for _, n := range notes {
		if n.Kind == kind {
			return n, true
		}
	}
	return Note{}, false
}

func TestCountsTheDay(t *testing.T) {
	s := store(t)
	s.Record(noon, Observation{Exit: 0})
	s.Record(noon, Observation{Exit: 1})
	s.Record(noon, Observation{Exit: 130}) // Ctrl-C is not a failure
	s.Record(noon, Observation{Exit: 0, Ignored: true})
	d := s.Days["2026-11-02"]
	if d.Commands != 4 || d.Failures != 1 {
		t.Errorf("commands %d, failures %d; want 4 and 1", d.Commands, d.Failures)
	}
}

func TestIgnoredCommandsCountAsActivityButNotOutcomes(t *testing.T) {
	s := store(t)
	s.Record(noon, Observation{Exit: 1, Ignored: true})
	d := s.Days["2026-11-02"]
	if d.Commands != 1 || d.Failures != 0 || d.LongestStreak != 0 {
		t.Errorf("an ignored command should only count as activity: %+v", d)
	}
}

func TestANewFailureRecordIsAnnouncedOnlyWhenItIsOneAndBigEnough(t *testing.T) {
	s := store(t)
	var got []string
	for streak := 0; streak < 7; streak++ {
		notes := s.Record(noon, Observation{Exit: 1, FailStreak: streak})
		if n, ok := has(notes, KindRecord); ok {
			got = append(got, string(rune('0'+n.N)))
		}
	}
	// The 5th, 6th and 7th failures in a row each beat the best so far.
	if strings.Join(got, ",") != "5,6,7" {
		t.Errorf("records announced at streaks %v, want 5,6,7", got)
	}
	// Equalling the best is not a record.
	if _, ok := has(s.Record(noon, Observation{Exit: 1, FailStreak: 6}), KindRecord); ok {
		t.Error("equalling the record should not be announced again")
	}
	if s.Lifetime.BestStreak != 7 {
		t.Errorf("best streak %d, want 7", s.Lifetime.BestStreak)
	}
}

func TestShortStreaksAreNeverRecords(t *testing.T) {
	s := store(t)
	for streak := 0; streak < 4; streak++ {
		if _, ok := has(s.Record(noon, Observation{Exit: 1, FailStreak: streak}), KindRecord); ok {
			t.Fatalf("a streak of %d is not worth announcing", streak+1)
		}
	}
}

func TestReliefAfterARoughPatch(t *testing.T) {
	s := store(t)
	if _, ok := has(s.Record(noon, Observation{Exit: 0, FailStreak: 2}), KindRelief); ok {
		t.Error("two failures is not a rough patch")
	}
	n, ok := has(s.Record(noon, Observation{Exit: 0, FailStreak: 4}), KindRelief)
	if !ok || n.N != 4 {
		t.Errorf("relief after 4 failures: %+v, %v", n, ok)
	}
}

func TestReliefIsAnnouncedAtMostThreeTimesADayButAllAreCounted(t *testing.T) {
	s := store(t)
	said := 0
	for i := 0; i < 5; i++ {
		if _, ok := has(s.Record(noon, Observation{Exit: 0, FailStreak: 3}), KindRelief); ok {
			said++
		}
	}
	if said != 3 {
		t.Errorf("announced %d reliefs, want 3", said)
	}
	if s.Days["2026-11-02"].Reliefs != 5 {
		t.Errorf("counted %d reliefs, want all 5", s.Days["2026-11-02"].Reliefs)
	}
}

func TestLateNightsInARow(t *testing.T) {
	s := store(t)
	night := func(day int) time.Time { return time.Date(2026, 11, day, 2, 0, 0, 0, time.Local) }
	for day := 1; day <= 2; day++ {
		if _, ok := has(s.Record(night(day), Observation{Exit: 0, Hour: 2}), KindLateRun); ok {
			t.Fatalf("night %d is too early to remark on", day)
		}
	}
	n, ok := has(s.Record(night(3), Observation{Exit: 0, Hour: 2}), KindLateRun)
	if !ok || n.N != 3 {
		t.Errorf("third night in a row: %+v, %v", n, ok)
	}
	// Said once that day, however many late commands follow.
	if _, ok := has(s.Record(night(3), Observation{Exit: 0, Hour: 3}), KindLateRun); ok {
		t.Error("the same night must not be announced twice")
	}
	// A night off breaks the run.
	if _, ok := has(s.Record(night(5), Observation{Exit: 0, Hour: 2}), KindLateRun); ok {
		t.Error("a gap day should have broken the run")
	}
}

func TestComebackAfterAWayButNotTheFirstTimeEver(t *testing.T) {
	s := store(t)
	if _, ok := has(s.Record(noon, Observation{}), KindComeback); ok {
		t.Error("the very first command is not a comeback")
	}
	if _, ok := has(s.Record(noon.Add(2*time.Hour), Observation{}), KindComeback); ok {
		t.Error("two hours is not being away")
	}
	n, ok := has(s.Record(noon.Add(2*time.Hour+30*time.Hour), Observation{}), KindComeback)
	if !ok || n.N != 30 {
		t.Errorf("after 30 hours away: %+v, %v", n, ok)
	}
}

func TestBusyAndFirstPushOncePerDay(t *testing.T) {
	s := store(t)
	busy := 0
	for i := 0; i < 130; i++ {
		if _, ok := has(s.Record(noon, Observation{Exit: 0}), KindBusy); ok {
			busy++
		}
	}
	if busy != 1 {
		t.Errorf("announced busy %d times, want once", busy)
	}
	pushes := 0
	for i := 0; i < 4; i++ {
		if _, ok := has(s.Record(noon, Observation{Exit: 0, Push: true}), KindFirstPush); ok {
			pushes++
		}
	}
	if pushes != 1 || s.Days["2026-11-02"].Pushes != 4 {
		t.Errorf("announced %d times, counted %d pushes; want 1 and 4", pushes, s.Days["2026-11-02"].Pushes)
	}
	// A new day starts fresh.
	if _, ok := has(s.Record(noon.AddDate(0, 0, 1), Observation{Exit: 0, Push: true}), KindFirstPush); !ok {
		t.Error("the next day's first push should be announced")
	}
}

func TestTheMostImportantMomentComesFirst(t *testing.T) {
	s := store(t)
	// A day with many commands, and then a relief and a record can't both happen
	// from one command, but a relief and a first push can.
	notes := s.Record(noon, Observation{Exit: 0, Push: true, FailStreak: 5})
	got := kinds(notes)
	if len(got) != 2 || got[0] != KindRelief || got[1] != KindFirstPush {
		t.Errorf("order %v, want relief then first_push", got)
	}
}

func TestSavingAndLoadingKeepsEverything(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "memory.json")
	s := Load(path)
	s.Record(noon, Observation{Exit: 1, FailStreak: 5, Hour: 2})
	if !s.Dirty() {
		t.Error("a recorded command should mark the memory unsaved")
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if s.Dirty() {
		t.Error("saving should clear the unsaved mark")
	}
	back := Load(path)
	if back.Days["2026-11-02"].Failures != 1 || back.Lifetime.BestStreak != 6 || back.Lifetime.FirstSeen != "2026-11-02" {
		t.Errorf("did not round trip: %+v %+v", back.Days["2026-11-02"], back.Lifetime)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("memory file mode %v, want 0600", info.Mode().Perm())
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("temporary file left behind")
	}
}

func TestAnUnreadableFileIsReplacedNotTrusted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	os.WriteFile(path, []byte("not json at all"), 0o600)
	s := Load(path)
	s.Record(noon, Observation{})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if Load(path).Days["2026-11-02"] == nil {
		t.Error("the memory should have been rewritten")
	}
}

// The promise: only counts are kept. This checks the saved file has no text
// fields beyond the day keys and the announced kinds.
func TestTheFileHoldsOnlyCountsAndNeverACommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := Load(path)
	for i := 0; i < 5; i++ {
		s.Record(noon, Observation{Exit: 1, FailStreak: i, Push: i == 4})
	}
	s.Save()
	data, _ := os.ReadFile(path)
	var raw map[string]any
	json.Unmarshal(data, &raw)
	days := raw["days"].(map[string]any)
	for date, v := range days {
		for key, val := range v.(map[string]any) {
			switch val.(type) {
			case float64: // a count
			case []any: // the kinds announced today
				for _, k := range val.([]any) {
					if _, isString := k.(string); !isString {
						t.Errorf("%s.%s holds something odd", date, key)
					}
				}
			default:
				t.Errorf("%s.%s is %T, but only counts are allowed", date, key, val)
			}
		}
	}
	if strings.Contains(string(data), "git") || strings.Contains(string(data), "go test") {
		t.Error("a command appeared in the saved memory")
	}
}

func TestOldDaysAreForgotten(t *testing.T) {
	s := store(t)
	old := noon.AddDate(0, 0, -(KeepDays + 5))
	s.Record(old, Observation{Exit: 0})
	s.Record(noon, Observation{Exit: 0})
	if _, ok := s.Days[dayKey(old)]; ok {
		t.Error("a day older than the limit should have been dropped")
	}
	if _, ok := s.Days[dayKey(noon)]; !ok {
		t.Error("today must be kept")
	}
}

func TestSummaryAddsUpTheWeek(t *testing.T) {
	s := store(t)
	// Monday: 3 commands, 1 failure. Sunday: 2 commands with a 4-long streak.
	for i := 0; i < 3; i++ {
		s.Record(noon, Observation{Exit: i % 2, Hour: 12})
	}
	sunday := noon.AddDate(0, 0, -1)
	s.Record(sunday, Observation{Exit: 1, FailStreak: 3, Hour: 2})
	s.Record(sunday, Observation{Exit: 0, FailStreak: 4, Push: true})
	w := s.Summary(noon, 7)
	if w.ActiveDays != 2 || w.Commands != 5 || w.Failures != 2 {
		t.Errorf("days %d commands %d failures %d; want 2, 5, 2", w.ActiveDays, w.Commands, w.Failures)
	}
	if w.LongestStreak != 4 || w.LongestOn != "Sunday" || w.Reliefs != 1 || w.Pushes != 1 || w.LateNights != 1 {
		t.Errorf("summary wrong: %+v", w)
	}
	if w.BusiestDay != "Monday" || w.BusiestCount != 3 {
		t.Errorf("busiest %s %d, want Monday 3", w.BusiestDay, w.BusiestCount)
	}
	// Looking back only one day sees only today.
	if one := s.Summary(noon, 1); one.ActiveDays != 1 || one.Commands != 3 {
		t.Errorf("a one-day summary should only see today: %+v", one)
	}
}
