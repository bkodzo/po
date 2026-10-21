package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bkodzo/po/internal/config"
	"github.com/bkodzo/po/internal/event"
	"github.com/bkodzo/po/internal/ipc"
	"github.com/bkodzo/po/internal/pet"
	"github.com/bkodzo/po/internal/stack"
	"github.com/bkodzo/po/internal/theme"
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

func startDaemon(t *testing.T) (stop func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "d")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PO_SOCK", dir+"/s")
	t.Setenv("HOME", dir)
	l, err := ipc.Listen()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Reader = "rules"
	st := stack.Build(cfg)
	p := pet.New()
	p.MinConfidence = st.MinConfidence
	done := make(chan struct{})
	go func() { Run(theme.Soccer, p, st, l); close(done) }()
	return func() {
		ipc.Send(ipc.Message{Op: "stop"})
		<-done
		l.Close()
		os.RemoveAll(dir)
	}
}

func readState(t *testing.T, sub *ipc.Subscription) State {
	t.Helper()
	raw, err := sub.Next()
	if err != nil {
		t.Fatal(err)
	}
	var st State
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("bad state line %q: %v", raw, err)
	}
	return st
}

func TestSubscriberSeesAReactionToACommand(t *testing.T) {
	defer startDaemon(t)()

	sub, err := ipc.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	first := readState(t, sub)
	if first.V != StateVersion || first.Mood != "calm" || first.Theme != "soccer" || first.LineSeq != 0 {
		t.Fatalf("first state wrong: %+v", first)
	}

	if err := ipc.Send(ipc.Message{Cmd: &event.Command{Text: "go test ./...", Exit: 1}}); err != nil {
		t.Fatal(err)
	}
	var got State
	deadline := time.After(5 * time.Second)
	for got.Mood != "frustrated" {
		select {
		case <-deadline:
			t.Fatalf("never saw frustrated; last state %+v", got)
		default:
		}
		got = readState(t, sub)
	}
	if got.Line == "" || got.LineSeq != 1 {
		t.Errorf("a reaction should come with a new line: %+v", got)
	}
}

func TestLateSubscriberIsCorrectImmediately(t *testing.T) {
	defer startDaemon(t)()
	ipc.Send(ipc.Message{Cmd: &event.Command{Text: "make", Exit: 1}})
	time.Sleep(600 * time.Millisecond)

	sub, err := ipc.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if first := readState(t, sub); first.Mood != "frustrated" {
		t.Errorf("a late subscriber should start with the current state, got %+v", first)
	}
}

func TestDeadSubscribersAreForgotten(t *testing.T) {
	defer startDaemon(t)()
	sub, err := ipc.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	readState(t, sub)
	sub.Close()
	// Changes make the daemon try to write to the closed connection.
	for i := 0; i < 3; i++ {
		ipc.Send(ipc.Message{Cmd: &event.Command{Text: "make", Exit: 1}})
		time.Sleep(400 * time.Millisecond)
	}
	raw, err := ipc.Ask(ipc.Message{Op: "status"})
	if err != nil {
		t.Fatal(err)
	}
	var s Status
	json.Unmarshal(raw, &s)
	if s.Subscribers != 0 {
		t.Errorf("subscribers = %d, want 0 after the client left", s.Subscribers)
	}
}

func TestThemeIsExportedNextToTheSocket(t *testing.T) {
	defer startDaemon(t)()
	sock, _ := ipc.Path()
	path := filepath.Join(filepath.Dir(sock), "theme.json")
	for i := 0; i < 40; i++ { // the daemon writes it as it starts
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("theme.json was not written to %s", path)
}
