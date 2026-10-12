package modelserver

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// fakeServer writes a stand-in for llama-server that answers /health after a
// short delay, and returns its path and a free port.
func fakeServer(t *testing.T) (binary, model, port string) {
	t.Helper()
	dir := t.TempDir()
	binary = filepath.Join(dir, "fake-llama-server")
	script := `#!/usr/bin/env python3
import sys, time
from http.server import BaseHTTPRequestHandler, HTTPServer
port = int(sys.argv[sys.argv.index("--port") + 1])
time.sleep(0.4)
class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_GET(self):
        self.send_response(200); self.end_headers(); self.wfile.write(b'{"status":"ok"}')
HTTPServer(("127.0.0.1", port), H).serve_forever()
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	model = filepath.Join(dir, "model.gguf")
	os.WriteFile(model, []byte("x"), 0o644)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port = strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	ln.Close()
	return binary, model, port
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestStartsOnDemandAndStopsWhenIdle(t *testing.T) {
	bin, model, port := fakeServer(t)
	s, err := New(bin, model, "http://127.0.0.1:"+port, filepath.Join(t.TempDir(), "pid"), 600*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	s.CheckEvery = 100 * time.Millisecond
	defer s.Stop()

	if s.healthy() {
		t.Fatal("server should not be running before it is wanted")
	}
	if s.Use() {
		t.Error("the first call should report not ready while the server starts")
	}
	waitFor(t, "the server to become healthy", s.healthy)
	if !s.Use() {
		t.Error("once healthy, Use should report ready")
	}

	stop := make(chan struct{})
	defer close(stop)
	go s.Watch(stop)
	waitFor(t, "the idle server to be shut down", func() bool { return !s.healthy() })
}

func TestUseKeepsTheServerAlive(t *testing.T) {
	bin, model, port := fakeServer(t)
	s, _ := New(bin, model, "http://127.0.0.1:"+port, "", 700*time.Millisecond)
	s.CheckEvery = 100 * time.Millisecond
	defer s.Stop()
	s.Use()
	waitFor(t, "healthy", s.healthy)
	stop := make(chan struct{})
	defer close(stop)
	go s.Watch(stop)
	for i := 0; i < 15; i++ { // 1.5s of regular use, longer than the idle limit
		s.Use()
		time.Sleep(100 * time.Millisecond)
	}
	if !s.healthy() {
		t.Error("a server in regular use should not be shut down")
	}
}

func TestLeavesAnExternalServerAlone(t *testing.T) {
	_, model, _ := fakeServer(t)
	srv := http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"ok"}`)) })}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	go srv.Serve(ln)
	defer srv.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)

	s, err := New("/bin/true", model, "http://127.0.0.1:"+port, "", 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	s.CheckEvery = 50 * time.Millisecond
	if !s.Use() {
		t.Error("an external healthy server counts as ready")
	}
	stop := make(chan struct{})
	defer close(stop)
	go s.Watch(stop)
	time.Sleep(600 * time.Millisecond)
	s.Stop()
	if !s.healthy() {
		t.Error("Po shut down a server it did not start")
	}
	if got := s.State(); got != "running (not started by Po, left alone)" {
		t.Errorf("state = %q", got)
	}
}

func TestReportsAnOccupiedPort(t *testing.T) {
	bin, model, _ := fakeServer(t)
	ln, _ := net.Listen("tcp", "127.0.0.1:0") // accepts connections but never answers /health
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	s, _ := New(bin, model, "http://127.0.0.1:"+port, "", time.Minute)
	if s.Use() {
		t.Error("should not be ready")
	}
	if got := s.State(); got != fmt.Sprintf("unavailable: something else is using port %s", port) {
		t.Errorf("state = %q", got)
	}
}

func TestNewRejectsMissingThings(t *testing.T) {
	bin, model, port := fakeServer(t)
	if _, err := New(bin, "/no/such/model.gguf", "http://127.0.0.1:"+port, "", time.Minute); err == nil {
		t.Error("missing model file should be an error")
	}
	if _, err := New(bin, model, "http://127.0.0.1", "", time.Minute); err == nil {
		t.Error("an address with no port should be an error")
	}
}

func TestKillsAStaleServerFromAnEarlierRun(t *testing.T) {
	bin, model, port := fakeServer(t)
	pidFile := filepath.Join(t.TempDir(), "pid")
	first, _ := New(bin, model, "http://127.0.0.1:"+port, pidFile, time.Minute)
	first.Use()
	waitFor(t, "healthy", first.healthy)
	// Simulate a crash: the first supervisor vanishes without calling Stop.
	if _, err := New(bin, model, "http://127.0.0.1:"+port, pidFile, time.Minute); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the stale server to be killed", func() bool { return !first.healthy() })
}
