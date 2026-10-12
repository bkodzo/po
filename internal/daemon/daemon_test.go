package daemon

import (
	"testing"
	"time"

	"github.com/bkodzo/po/internal/event"
)

func TestFailStreakCountsFailuresInARow(t *testing.T) {
	d := &Daemon{}
	fail := event.Command{Text: "go test", Exit: 1}
	pass := event.Command{Text: "go test", Exit: 0}

	if got := d.eventFor(fail).FailStreak; got != 0 {
		t.Errorf("first failure sees streak %d, want 0", got)
	}
	if got := d.eventFor(fail).FailStreak; got != 1 {
		t.Errorf("second failure sees streak %d, want 1", got)
	}
	if got := d.eventFor(fail).FailStreak; got != 2 {
		t.Errorf("third failure sees streak %d, want 2", got)
	}
	if got := d.eventFor(pass).FailStreak; got != 3 {
		t.Errorf("the success should see the 3 failures before it, got %d", got)
	}
	if got := d.eventFor(pass).FailStreak; got != 0 {
		t.Errorf("after a success the streak should reset, got %d", got)
	}
}

func TestCtrlCDoesNotCountAsFailure(t *testing.T) {
	d := &Daemon{}
	d.eventFor(event.Command{Text: "go run .", Exit: 130})
	d.eventFor(event.Command{Text: "go run .", Exit: 130})
	if d.failStreak != 0 {
		t.Errorf("interrupts counted as failures: streak %d", d.failStreak)
	}
}

func TestStreakEndsAfterALongQuiet(t *testing.T) {
	d := &Daemon{}
	d.eventFor(event.Command{Text: "make", Exit: 1})
	d.eventFor(event.Command{Text: "make", Exit: 1})
	d.lastCmd = time.Now().Add(-streakResets - time.Minute)
	if got := d.eventFor(event.Command{Text: "make", Exit: 1}).FailStreak; got != 0 {
		t.Errorf("a failure after a long break should start a new streak, got %d", got)
	}
}
