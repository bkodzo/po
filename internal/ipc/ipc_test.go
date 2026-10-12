package ipc

import (
	"os"
	"testing"
	"time"

	"github.com/bkodzo/po/internal/event"
)

func useShortSocket(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("", "c")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("PO_SOCK", dir+"/s")
}

func TestSendIsAcknowledgedAfterHandling(t *testing.T) {
	useShortSocket(t)
	l, err := Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	handled := make(chan Message, 1)
	go func() {
		req := <-l.C
		time.Sleep(100 * time.Millisecond)
		handled <- req.Message
		req.Ack()
	}()

	start := time.Now()
	err = Send(Message{TTY: "/dev/ttys001", Cmd: &event.Command{Text: "go test", Exit: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Error("Send returned before the pet handled the message")
	}
	m := <-handled
	if m.Cmd == nil || m.Cmd.Text != "go test" || m.Cmd.Exit != 1 || m.TTY != "/dev/ttys001" {
		t.Errorf("got %+v", m)
	}
}

func TestUnansweredSendTimesOut(t *testing.T) {
	useShortSocket(t)
	l, err := Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	start := time.Now()
	if err := Send(Message{Op: "hide"}); err == nil {
		t.Error("expected a timeout when nothing acknowledges")
	}
	if time.Since(start) > time.Second {
		t.Error("send blocked too long")
	}
}

func TestSecondListenerIsRefused(t *testing.T) {
	useShortSocket(t)
	l, err := Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if _, err := Listen(); err != ErrRunning {
		t.Errorf("got %v, want ErrRunning", err)
	}
}

func TestSendWithNoPetFailsFast(t *testing.T) {
	useShortSocket(t)
	start := time.Now()
	if err := Send(Message{Op: "show"}); err == nil {
		t.Error("expected an error with no pet running")
	}
	if time.Since(start) > time.Second {
		t.Error("send blocked too long")
	}
}

func TestAskReturnsData(t *testing.T) {
	useShortSocket(t)
	l, err := Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		req := <-l.C
		req.Reply(map[string]any{"mood": "calm", "n": 3})
	}()
	raw, err := Ask(Message{Op: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"mood":"calm","n":3}` {
		t.Errorf("got %s", raw)
	}
}
