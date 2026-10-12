// Package modelserver starts and stops a local model server on demand, so the
// model only uses memory while it is being used.
package modelserver

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bkodzo/po/internal/mood"
)

// ErrWarming means the model is not ready yet, so the caller should answer
// without it this time.
var ErrWarming = errors.New("the model is starting")

// Supervisor runs llama-server for a model file, but only when asked, and shuts
// it down after a quiet spell. A server it did not start is never touched.
type Supervisor struct {
	Binary  string
	Model   string
	Port    string
	Idle    time.Duration
	PIDFile string
	// CheckEvery is how often Watch looks for an idle server.
	CheckEvery time.Duration

	mu      sync.Mutex
	cmd     *exec.Cmd
	lastUse time.Time
	failure string
}

// New checks that the server program and model file exist and that the address
// is a local port Po can start a server on. It also removes any server left
// over from an earlier run that was killed without cleaning up.
func New(binary, model, rawURL, pidFile string, idle time.Duration) (*Supervisor, error) {
	if binary == "" {
		for _, c := range []string{"llama-server", "/opt/homebrew/bin/llama-server", "/usr/local/bin/llama-server"} {
			if p, err := exec.LookPath(c); err == nil {
				binary = p
				break
			}
		}
	}
	if binary == "" {
		return nil, errors.New("llama-server not found (install llama.cpp, for example: brew install llama.cpp)")
	}
	if info, err := os.Stat(model); err != nil || info.IsDir() {
		return nil, fmt.Errorf("model file %q not found", model)
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Port() == "" {
		return nil, fmt.Errorf("model address %q needs a port to start a server on", rawURL)
	}
	s := &Supervisor{Binary: binary, Model: model, Port: u.Port(), Idle: idle, PIDFile: pidFile, CheckEvery: 10 * time.Second}
	s.killStale()
	return s, nil
}

func (s *Supervisor) healthy() bool {
	client := http.Client{Timeout: 400 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:" + s.Port + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *Supervisor) portBusy() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+s.Port, 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Use records that the model is wanted. It reports whether it is ready now; if
// it is not, it begins starting it and the caller should carry on without it.
func (s *Supervisor) Use() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastUse = time.Now()
	if s.healthy() {
		return true
	}
	if s.cmd != nil {
		return false // already starting
	}
	if s.portBusy() {
		s.failure = "something else is using port " + s.Port
		return false
	}
	s.start()
	return false
}

func (s *Supervisor) start() {
	cmd := exec.Command(s.Binary, "-m", s.Model, "--host", "127.0.0.1", "--port", s.Port, "-ngl", "99", "-c", "4096")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		s.failure = err.Error()
		return
	}
	s.failure = ""
	s.cmd = cmd
	if s.PIDFile != "" {
		os.WriteFile(s.PIDFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600)
	}
	go func() {
		cmd.Wait()
		s.mu.Lock()
		if s.cmd == cmd {
			s.cmd = nil
			s.failure = "the model server exited"
		}
		s.mu.Unlock()
		if s.PIDFile != "" {
			os.Remove(s.PIDFile)
		}
	}()
}

// State describes the server for `po status`.
func (s *Supervisor) State() string {
	s.mu.Lock()
	cmd, failure := s.cmd, s.failure
	s.mu.Unlock()
	switch {
	case s.healthy() && cmd != nil:
		return "running (started by Po, stops after " + s.Idle.String() + " idle)"
	case s.healthy():
		return "running (not started by Po, left alone)"
	case cmd != nil:
		return "starting"
	case failure != "":
		return "unavailable: " + failure
	}
	return "stopped (Po starts it when needed)"
}

// Watch shuts the server down after it has been idle, until stop is closed.
func (s *Supervisor) Watch(stop <-chan struct{}) {
	t := time.NewTicker(s.CheckEvery)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			s.mu.Lock()
			idle := s.cmd != nil && time.Since(s.lastUse) > s.Idle
			s.mu.Unlock()
			if idle {
				s.Stop()
			}
		}
	}
}

// Stop shuts down a server Po started. It does nothing otherwise.
func (s *Supervisor) Stop() {
	s.mu.Lock()
	cmd := s.cmd
	s.cmd = nil
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			if syscall.Kill(cmd.Process.Pid, 0) != nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		close(done)
	}()
	<-done
	if syscall.Kill(cmd.Process.Pid, 0) == nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	if s.PIDFile != "" {
		os.Remove(s.PIDFile)
	}
}

// killStale stops a server this program started in an earlier run.
func (s *Supervisor) killStale() {
	if s.PIDFile == "" {
		return
	}
	data, err := os.ReadFile(s.PIDFile)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	os.Remove(s.PIDFile)
	if err != nil || pid <= 1 {
		return
	}
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil || !strings.Contains(string(out), s.Model) {
		return // not ours any more
	}
	syscall.Kill(pid, syscall.SIGTERM)
}

// Managed asks a model through a supervisor, so the model is started when it is
// wanted and the caller is told to carry on without it while it loads.
type Managed struct {
	Inner mood.Reader
	Sup   *Supervisor
}

func (m Managed) Read(e mood.Event) (mood.Reading, error) {
	if !m.Sup.Use() {
		return mood.Reading{}, ErrWarming
	}
	return m.Inner.Read(e)
}
