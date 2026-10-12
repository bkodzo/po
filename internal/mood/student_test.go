package mood

import (
	"math"
	"testing"
)

func TestNoStudentUntilOneIsTrained(t *testing.T) {
	// The placeholder model file has no weights, so there is no student.
	if s, err := ParseStudent([]byte(`{"dim":0}`)); err != nil || s.Dim != 0 {
		t.Fatalf("placeholder should parse as empty, got %+v, %v", s, err)
	}
}

func TestFeaturesAreStableAndBounded(t *testing.T) {
	e := Event{Kind: "command", Cmd: "go test ./...", Exit: 1, Millis: 900, FailStreak: 2, Hour: 14}
	a, b := Features(e, 1024), Features(e, 1024)
	if len(a) == 0 || len(a) != len(b) {
		t.Fatalf("features differ between calls: %v vs %v", a, b)
	}
	for i := range a {
		if a[i] != b[i] || a[i] < 0 || a[i] >= 1024 {
			t.Errorf("feature %d = %d is out of range or unstable", i, a[i])
		}
	}
}

// target is a teacher distribution that puts everything on one mood.
func target(m Mood) [5]float64 {
	var t [5]float64
	for i, x := range All {
		if x == m {
			t[i] = 1
		}
	}
	return t
}

func TestStudentLearnsASimpleRule(t *testing.T) {
	// Teach it: failures are frustrating, successes after a streak are a relief,
	// quiet successes are calm.
	var samples []Sample
	for i := 0; i < 200; i++ {
		fail := Event{Kind: "command", Cmd: "make", Exit: 1, Millis: 500, FailStreak: i % 3, Hour: 14}
		relief := Event{Kind: "command", Cmd: "make", Exit: 0, Millis: 500, FailStreak: 3, Hour: 14}
		quiet := Event{Kind: "command", Cmd: "make", Exit: 0, Millis: 500, FailStreak: 0, Hour: 14}
		samples = append(samples,
			Sample{fail, target(Frustrated)},
			Sample{relief, target(Celebrating)},
			Sample{quiet, target(Calm)},
		)
	}
	s := TrainStudent(samples, 512, 20, 0.2, 1e-4, 1)
	check := func(e Event, want Mood) {
		t.Helper()
		r, _ := s.Read(e)
		if got, conf := r.Top(); got != want || conf < 0.8 {
			t.Errorf("%+v: got %s %.2f, want %s with confidence", e, got, conf, want)
		}
	}
	check(Event{Kind: "command", Cmd: "make", Exit: 1, Millis: 500, FailStreak: 1, Hour: 14}, Frustrated)
	check(Event{Kind: "command", Cmd: "make", Exit: 0, Millis: 500, FailStreak: 3, Hour: 14}, Celebrating)
	check(Event{Kind: "command", Cmd: "make", Exit: 0, Millis: 500, FailStreak: 0, Hour: 14}, Calm)
}

func TestMarshalRoundTrip(t *testing.T) {
	s := &Student{Dim: 64, Bias: [5]float64{0.1, -0.2, 0.3, 0, 0}, W: map[int][5]float64{
		3: {0.5, -0.5, 0.25, 0, 0}, 9: {0.001, 0, 0, 0, 0}, // the second is too small to keep
	}}
	data, err := s.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseStudent(data)
	if err != nil {
		t.Fatal(err)
	}
	if back.Dim != 64 || math.Abs(back.W[3][0]-0.5) > 1e-3 || math.Abs(back.Bias[2]-0.3) > 1e-3 {
		t.Errorf("round trip lost data: %+v", back)
	}
	if _, kept := back.W[9]; kept {
		t.Error("tiny weights should be dropped")
	}
}

func TestStudentFallsBackForTextEvents(t *testing.T) {
	s := &Student{Dim: 64, W: map[int][5]float64{}}
	r, err := s.Read(Event{Kind: "text", Text: "ugh the build failed again"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Top(); got != Frustrated {
		t.Errorf("got %s", got)
	}
}
