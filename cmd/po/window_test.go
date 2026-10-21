package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeWindow writes a stand-in program that idles until told to stop, named like the real one
// so the check that a pid belongs to the window passes.
func fakeWindow(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "w")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { windowStop(); os.RemoveAll(dir) })
	path := filepath.Join(dir, windowProgram)
	if err := os.WriteFile(path, []byte("#!/bin/sh\ntrap 'exit 0' TERM\nwhile :; do sleep 0.2; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PO_SOCK", filepath.Join(dir, "s"))
	t.Setenv("PO_WINDOW", path)
	t.Setenv("PO_NO_AUTOSTART", "1")
	return path
}

func TestWindowStartStatusStop(t *testing.T) {
	fakeWindow(t)

	if got := windowStatus(); !strings.Contains(got, "not running") {
		t.Errorf("status before start = %q", got)
	}
	msg, err := windowStart()
	if err != nil || !strings.Contains(msg, "window started") {
		t.Fatalf("start = %q, %v", msg, err)
	}
	if got := windowStatus(); !strings.Contains(got, "is running") {
		t.Errorf("status after start = %q", got)
	}
	again, _ := windowStart()
	if !strings.Contains(again, "already running") {
		t.Errorf("a second start should say it is already running, got %q", again)
	}
	if got := windowStop(); !strings.Contains(got, "stopped") {
		t.Errorf("stop = %q", got)
	}
	time.Sleep(200 * time.Millisecond)
	if got := windowStatus(); !strings.Contains(got, "not running") {
		t.Errorf("status after stop = %q", got)
	}
}

func TestStopWhenNothingIsRunning(t *testing.T) {
	fakeWindow(t)
	if got := windowStop(); !strings.Contains(got, "not running") {
		t.Errorf("got %q", got)
	}
}

func TestMissingProgramGivesAHelpfulError(t *testing.T) {
	fakeWindow(t)
	t.Setenv("PO_WINDOW", "/no/such/dir/"+windowProgram)
	_, err := windowStart()
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestStalePidFileIsNotMistakenForTheWindow(t *testing.T) {
	fakeWindow(t)
	pidFile, _, _ := windowFiles()
	os.MkdirAll(filepath.Dir(pidFile), 0o755)
	// pid 1 is never ours, and our own pid is not the window.
	for _, pid := range []string{"1", "999999", "not a number"} {
		os.WriteFile(pidFile, []byte(pid), 0o644)
		if _, ok := runningWindow(); ok {
			t.Errorf("pid file %q was taken for a running window", pid)
		}
	}
	os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644)
	if _, ok := runningWindow(); ok {
		t.Error("the test process was mistaken for the window")
	}
}
