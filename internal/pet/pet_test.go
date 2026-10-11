package pet

import (
	"testing"
	"time"

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

func TestMoodFadesBackToCalm(t *testing.T) {
	p := New()
	for i := 0; i < 5; i++ {
		p.Apply(reading(mood.Frustrated, 0.9))
	}
	if p.Mood != mood.Frustrated {
		t.Fatalf("setup: mood is %s", p.Mood)
	}
	p.Decay(30 * time.Second)
	if p.Mood != mood.Frustrated {
		t.Errorf("30s of quiet should not be enough, mood is %s", p.Mood)
	}
	p.Decay(10 * time.Minute)
	if p.Mood != mood.Calm {
		t.Errorf("10 minutes of quiet should leave it calm, mood is %s", p.Mood)
	}
}
