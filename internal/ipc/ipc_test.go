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

func TestSendReachesListener(t *testing.T) {
	useShortSocket(t)
	l, err := Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	if err := Send(event.Command{Text: "go test", Exit: 1}); err != nil {
		t.Fatal(err)
	}
	select {
	case c := <-l.C:
		if c.Text != "go test" || c.Exit != 1 {
			t.Errorf("got %+v", c)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("command never arrived")
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
	if err := Send(event.Command{Text: "x"}); err == nil {
		t.Error("expected an error with no pet running")
	}
	if time.Since(start) > time.Second {
		t.Error("send blocked too long")
	}
}
