package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bkodzo/po/internal/event"
)

// Message is what a shell hook tells the pet. Op is "cmd" (the default), "show",
// "hide" or "stop". A message with a TTY and no Op also means the shell is
// waiting at a prompt on that terminal.
type Message struct {
	Op  string         `json:"op,omitempty"`
	TTY string         `json:"tty,omitempty"`
	Cmd *event.Command `json:"cmd,omitempty"`
}

// Request is a received message. Ack tells the sender it has been handled.
type Request struct {
	Message
	conn net.Conn
}

func (r Request) Ack() { r.Reply(nil) }

// Reply acknowledges the request and, if v is not nil, sends it back as JSON.
func (r Request) Reply(v any) {
	line := []byte("ok")
	if v != nil {
		if data, err := json.Marshal(v); err == nil {
			line = append(append(line, ' '), data...)
		}
	}
	r.conn.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
	r.conn.Write(append(line, '\n'))
}

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

const ackWait = 300 * time.Millisecond

// Send hands a message to a running pet and waits briefly for it to be
// handled. It gives up quickly, so a missing pet never slows the shell down.
func Send(m Message) error {
	_, err := Ask(m)
	return err
}

// Ask is Send for messages that get data back, such as a status request.
func Ask(m Message) (json.RawMessage, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
	if err := json.NewEncoder(conn).Encode(m); err != nil {
		return nil, err
	}
	conn.SetReadDeadline(time.Now().Add(ackWait))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return nil, err
	}
	if rest := strings.TrimSpace(strings.TrimPrefix(line, "ok")); rest != "" {
		return json.RawMessage(rest), nil
	}
	return nil, nil
}

type Listener struct {
	C    <-chan Request
	ln   net.Listener
	path string
}

var ErrRunning = errors.New("a pet is already running")

// Listen starts receiving messages from shell hooks.
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

	ch := make(chan Request, 16)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serve(conn, ch)
		}
	}()
	return &Listener{C: ch, ln: ln, path: path}, nil
}

func serve(conn net.Conn, ch chan<- Request) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var m Message
		if json.Unmarshal(scanner.Bytes(), &m) != nil {
			continue
		}
		req := Request{Message: m, conn: conn}
		select {
		case ch <- req:
		case <-time.After(time.Second):
			req.Ack()
		}
	}
}

func (l *Listener) Close() {
	l.ln.Close()
	os.Remove(l.path)
}
