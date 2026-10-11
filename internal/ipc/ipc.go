package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/bkodzo/po/internal/event"
)

// Path is where the pet listens. PO_SOCK overrides it, which also keeps
// tests under the short path limit unix sockets have.
func Path() (string, error) {
	if p := os.Getenv("PO_SOCK"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "po", "pet.sock"), nil
}

// Send hands a finished command to a running pet. It gives up quickly, so a
// missing pet never slows the shell down.
func Send(c event.Command) error {
	path, err := Path()
	if err != nil {
		return err
	}
	conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
	return json.NewEncoder(conn).Encode(c)
}

type Listener struct {
	C    <-chan event.Command
	ln   net.Listener
	path string
}

var ErrRunning = errors.New("a pet is already running")

// Listen starts receiving commands from shell hooks.
func Listen() (*Listener, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if conn, err := net.DialTimeout("unix", path, 200*time.Millisecond); err == nil {
		conn.Close()
		return nil, ErrRunning
	}
	os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	os.Chmod(path, 0o600)

	ch := make(chan event.Command, 16)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					var c event.Command
					if json.Unmarshal(scanner.Bytes(), &c) != nil {
						continue
					}
					select {
					case ch <- c:
					default:
					}
				}
			}()
		}
	}()
	return &Listener{C: ch, ln: ln, path: path}, nil
}

func (l *Listener) Close() {
	l.ln.Close()
	os.Remove(l.path)
}
