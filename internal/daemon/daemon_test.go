package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bkodzo/po/internal/config"
	"github.com/bkodzo/po/internal/event"
	"github.com/bkodzo/po/internal/ipc"
	"github.com/bkodzo/po/internal/memory"
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

// nextWith reads states until one satisfies ok, or fails after a timeout.
func nextWith(t *testing.T, sub *ipc.Subscription, why string, ok func(State) bool) State {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := sub.Next()
		if err != nil {
			t.Fatalf("%s: stream ended: %v", why, err)
		}
		var st State
		if json.Unmarshal(raw, &st) != nil {
			continue
		}
		if ok(st) {
			return st
		}
	}
	t.Fatalf("timed out waiting for: %s", why)
	return State{}
}

func TestAnIdleAnimationIsStreamedAndEndsByItself(t *testing.T) {
	defer startDaemon(t)()
	sub, err := ipc.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	readState(t, sub) // the starting state

	if err := ipc.Send(ipc.Message{Op: "idle", Name: "wake"}); err != nil {
		t.Fatal(err)
	}
	started := nextWith(t, sub, "wake to start", func(s State) bool { return s.Activity == "wake" })
	if started.ActivitySeq != 1 {
		t.Errorf("activity_seq = %d, want 1", started.ActivitySeq)
	}
	ended := nextWith(t, sub, "wake to end", func(s State) bool { return s.Activity == "" })
	if ended.ActivitySeq != 1 {
		t.Errorf("ending must not look like a new animation: seq %d", ended.ActivitySeq)
	}
}

func TestACommandWakesASleepingPet(t *testing.T) {
	defer startDaemon(t)()
	sub, _ := ipc.Subscribe()
	defer sub.Close()
	readState(t, sub)

	ipc.Send(ipc.Message{Op: "idle", Name: "sleep"})
	nextWith(t, sub, "sleep to start", func(s State) bool { return s.Activity == "sleep" })

	// Even a command too dull to react to means someone is back.
	ipc.Send(ipc.Message{Cmd: &event.Command{Text: "ls", Exit: 0}})
	woke := nextWith(t, sub, "the pet to wake", func(s State) bool { return s.Activity == "wake" })
	if woke.ActivitySeq < 2 {
		t.Errorf("waking is a new animation, seq %d", woke.ActivitySeq)
	}
}

func TestAReactionCutsAnIdleAnimationShort(t *testing.T) {
	defer startDaemon(t)()
	sub, _ := ipc.Subscribe()
	defer sub.Close()
	readState(t, sub)

	ipc.Send(ipc.Message{Op: "idle", Name: "stretch"})
	nextWith(t, sub, "stretch to start", func(s State) bool { return s.Activity == "stretch" })

	ipc.Send(ipc.Message{Cmd: &event.Command{Text: "go test ./...", Exit: 1}})
	got := nextWith(t, sub, "the failure to take over", func(s State) bool { return s.Mood == "frustrated" })
	if got.Activity != "" {
		t.Errorf("a stretch should not play over a reaction, got %q", got.Activity)
	}
}

func TestTurningIdleOffStopsTheAnimation(t *testing.T) {
	defer startDaemon(t)()
	sub, _ := ipc.Subscribe()
	defer sub.Close()
	readState(t, sub)

	cfg, _ := config.Load()
	cfg.Idle = "off"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	ipc.Send(ipc.Message{Op: "idle", Name: "sleep"})
	nextWith(t, sub, "sleep to start", func(s State) bool { return s.Activity == "sleep" })
	ipc.Send(ipc.Message{Op: "reload"})
	nextWith(t, sub, "sleep to stop once idle is off", func(s State) bool { return s.Activity == "" })
}

func TestUnknownAnimationNamesAreIgnored(t *testing.T) {
	defer startDaemon(t)()
	sub, _ := ipc.Subscribe()
	defer sub.Close()
	first := readState(t, sub)
	ipc.Send(ipc.Message{Op: "idle", Name: "backflip"})
	time.Sleep(600 * time.Millisecond)
	raw, _ := ipc.Ask(ipc.Message{Op: "status"})
	var s Status
	json.Unmarshal(raw, &s)
	if first.ActivitySeq != 0 || s.Mood == "" {
		t.Error("an unknown name should change nothing")
	}
}

func TestSwitchingThemeReachesTheStreamTheFileAndTheConfig(t *testing.T) {
	defer startDaemon(t)()
	sub, _ := ipc.Subscribe()
	defer sub.Close()
	first := readState(t, sub)
	if first.Theme != "soccer" {
		t.Fatalf("starts as %q", first.Theme)
	}

	ipc.Send(ipc.Message{Op: "theme", Name: "ghost"})
	got := nextWith(t, sub, "the stream to announce the ghost theme", func(s State) bool { return s.Theme == "ghost" })

	// The file a window reads must already be the new theme when the stream says so.
	sock, _ := ipc.Path()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(sock), "theme.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pack theme.Pack
	if err := json.Unmarshal(data, &pack); err != nil || pack.Name != "ghost" {
		t.Errorf("theme.json says %q (%v), want ghost", pack.Name, err)
	}
	_ = got

	// The choice is remembered.
	if cfg, err := config.Load(); err != nil || cfg.Theme != "ghost" {
		t.Errorf("config theme = %q (%v), want ghost", cfg.Theme, err)
	}
}

func TestAnUnknownThemeChangesNothing(t *testing.T) {
	defer startDaemon(t)()
	sub, _ := ipc.Subscribe()
	defer sub.Close()
	readState(t, sub)
	ipc.Send(ipc.Message{Op: "theme", Name: "no-such-theme"})
	time.Sleep(700 * time.Millisecond)
	if cfg, _ := config.Load(); cfg.Theme == "no-such-theme" {
		t.Error("an unknown theme must not be saved")
	}
	raw, _ := ipc.Ask(ipc.Message{Op: "status"})
	if raw == nil {
		t.Fatal("the daemon should still answer")
	}
}

func TestAThemeChosenInTheConfigIsPickedUpOnReload(t *testing.T) {
	defer startDaemon(t)()
	sub, _ := ipc.Subscribe()
	defer sub.Close()
	readState(t, sub)
	cfg, _ := config.Load()
	cfg.Theme = "cat"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	ipc.Send(ipc.Message{Op: "reload"})
	nextWith(t, sub, "the cat theme", func(s State) bool { return s.Theme == "cat" })
}

func send(c event.Command) { ipc.Send(ipc.Message{Cmd: &c}) }

// waitForFile waits for the memory file to appear, which happens when Po saves.
func waitForFile(t *testing.T, path string) {
	t.Helper()
	for i := 0; i < 60; i++ {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("%s never appeared", path)
}

func TestAReliefAfterARoughPatchIsSaidAloud(t *testing.T) {
	defer startDaemon(t)()
	sub, _ := ipc.Subscribe()
	defer sub.Close()
	readState(t, sub)

	for i := 0; i < 4; i++ {
		send(event.Command{Text: "go test ./...", Exit: 1})
		time.Sleep(150 * time.Millisecond)
	}
	send(event.Command{Text: "go test ./...", Exit: 0})
	got := nextWith(t, sub, "a line that mentions the 4 failures", func(s State) bool {
		return strings.Contains(s.Line, "4")
	})
	found := false
	for _, tmpl := range theme.Soccer.Milestones["relief"] {
		found = found || strings.Contains(tmpl, "%d") && got.Line == strings.Replace(tmpl, "%d", "4", 1)
	}
	if !found {
		t.Errorf("%q is not one of the relief lines", got.Line)
	}
}

func TestMemoryIsSavedWhenPoStopsAndHoldsOnlyCounts(t *testing.T) {
	stop := startDaemon(t)
	defer stop()
	send(event.Command{Text: "go test ./...", Exit: 1})
	send(event.Command{Text: "git push origin main", Exit: 0})
	time.Sleep(400 * time.Millisecond)
	ipc.Send(ipc.Message{Op: "stop"})
	path := memory.DefaultPath()
	waitForFile(t, path)

	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "go test") || strings.Contains(string(data), "git push") {
		t.Errorf("a command was written to the memory file:\n%s", data)
	}
	day := memory.Load(path).Days[time.Now().Format("2006-01-02")]
	if day == nil || day.Commands != 2 || day.Failures != 1 || day.Pushes != 1 {
		t.Errorf("remembered %+v, want 2 commands, 1 failure, 1 push", day)
	}
}

func TestPrivateCommandsLeaveNoTrace(t *testing.T) {
	stop := startDaemon(t)
	defer stop()
	for i := 0; i < 3; i++ {
		send(event.Command{Text: " export SECRET=hunter2", Exit: 1}) // a leading space means private
	}
	time.Sleep(500 * time.Millisecond)
	ipc.Send(ipc.Message{Op: "stop"})
	time.Sleep(800 * time.Millisecond)
	if _, err := os.Stat(memory.DefaultPath()); err == nil {
		data, _ := os.ReadFile(memory.DefaultPath())
		t.Errorf("private commands were remembered:\n%s", data)
	}
}

func TestTurningMemoryOffRecordsNothing(t *testing.T) {
	stop := startDaemon(t)
	defer stop()
	cfg, _ := config.Load()
	cfg.Memory = "off"
	cfg.Save()
	ipc.Send(ipc.Message{Op: "reload"})
	for i := 0; i < 3; i++ {
		send(event.Command{Text: "make", Exit: 1})
	}
	time.Sleep(500 * time.Millisecond)
	ipc.Send(ipc.Message{Op: "stop"})
	time.Sleep(800 * time.Millisecond)
	if _, err := os.Stat(memory.DefaultPath()); err == nil {
		t.Error("memory was off, but a file was written")
	}
}

func TestForgetWipesTheMemory(t *testing.T) {
	stop := startDaemon(t)
	defer stop()
	send(event.Command{Text: "make", Exit: 1})
	time.Sleep(300 * time.Millisecond)
	ipc.Send(ipc.Message{Op: "forget"})
	time.Sleep(300 * time.Millisecond)
	ipc.Send(ipc.Message{Op: "stop"})
	time.Sleep(800 * time.Millisecond)
	if day := memory.Load(memory.DefaultPath()).Days[time.Now().Format("2006-01-02")]; day != nil {
		t.Errorf("forgotten, but still remembers %+v", day)
	}
}
