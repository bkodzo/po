package mood

import (
	"errors"
	"testing"
)

type fake struct {
	top   Mood
	conf  float64
	err   error
	calls int
}

func (f *fake) Read(Event) (Reading, error) {
	f.calls++
	if f.err != nil {
		return Reading{}, f.err
	}
	rest := (1 - f.conf) / float64(len(All)-1)
	scores := map[Mood]float64{}
	for _, m := range All {
		scores[m] = rest
	}
	scores[f.top] = f.conf
	return Reading{Scores: scores}, nil
}

func TestRoutedSureFastLayerSkipsTheModel(t *testing.T) {
	fast, slow := &fake{top: Frustrated, conf: 0.8}, &fake{top: Tired, conf: 0.4}
	r := &Routed{Fast: fast, Slow: slow, Gate: 0.6}
	got, err := r.Read(Event{})
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := got.Top(); m != Frustrated || slow.calls != 0 {
		t.Errorf("mood %s, model calls %d; want frustrated and 0", m, slow.calls)
	}
	if s := r.Stats(); s.Fast != 1 || s.Slow != 0 {
		t.Errorf("stats %+v", s)
	}
}

func TestRoutedUnsureFastLayerAsksTheModel(t *testing.T) {
	fast, slow := &fake{top: Calm, conf: 0.4}, &fake{top: Tired, conf: 0.7}
	r := &Routed{Fast: fast, Slow: slow, Gate: 0.6}
	got, _ := r.Read(Event{})
	if m, _ := got.Top(); m != Tired {
		t.Errorf("got %s, want the model's answer (tired)", m)
	}
	if s := r.Stats(); s.Slow != 1 {
		t.Errorf("stats %+v", s)
	}
}

func TestRoutedFallsBackWhenTheModelFails(t *testing.T) {
	fast, slow := &fake{top: Calm, conf: 0.4}, &fake{err: errors.New("down")}
	r := &Routed{Fast: fast, Slow: slow, Gate: 0.6}
	got, err := r.Read(Event{})
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := got.Top(); m != Calm {
		t.Errorf("got %s, want the fast answer (calm)", m)
	}
	if s := r.Stats(); s.SlowFailed != 1 {
		t.Errorf("stats %+v", s)
	}
}

func TestRoutedWorksWithNoModel(t *testing.T) {
	fast := &fake{top: Calm, conf: 0.3}
	r := &Routed{Fast: fast, Gate: 0.6}
	if _, err := r.Read(Event{}); err != nil {
		t.Fatal(err)
	}
}

func TestRoutedUsesTheModelWhenTheFastLayerFails(t *testing.T) {
	fast, slow := &fake{err: errors.New("broken")}, &fake{top: Focused, conf: 0.5}
	r := &Routed{Fast: fast, Slow: slow, Gate: 0.6}
	got, err := r.Read(Event{})
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := got.Top(); m != Focused {
		t.Errorf("got %s", m)
	}
}

func TestRoutedFailsWhenBothFail(t *testing.T) {
	r := &Routed{Fast: &fake{err: errors.New("a")}, Slow: &fake{err: errors.New("b")}, Gate: 0.6}
	if _, err := r.Read(Event{}); err == nil {
		t.Error("expected an error")
	}
}
