package pet

import (
	"testing"

	"github.com/bkodzo/po/internal/mood"
)

func reading(m mood.Mood, conf float64) mood.Reading {
	return mood.Reading{Scores: map[mood.Mood]float64{m: conf}}
}

func TestUnsureReadingIgnored(t *testing.T) {
	p := New()
	if p.Apply(reading(mood.Frustrated, 0.2)) {
		t.Error("low confidence reading should be ignored")
	}
	if p.Mood != mood.Calm {
		t.Errorf("mood changed to %s", p.Mood)
	}
}

func TestMomentumResistsOneOffSwing(t *testing.T) {
	p := New()
	for i := 0; i < 5; i++ {
		p.Apply(reading(mood.Frustrated, 0.9))
	}
	p.Apply(reading(mood.Celebrating, 0.5))
	if p.Mood != mood.Frustrated {
		t.Errorf("one good event flipped the mood to %s", p.Mood)
	}
}
