package overlay

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// ValidTTY reports whether path is a terminal device owned by this user. The
// pet is told which terminal to draw on over a socket, so it must not trust
// the path blindly.
func ValidTTY(path string) bool {
	if !strings.HasPrefix(path, "/dev/tty") && !strings.HasPrefix(path, "/dev/pts/") {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}

// Target is one terminal the creature is drawn in the top-right corner of.
type Target struct {
	f      *os.File
	drawn  bool
	lastX  int
	width  int
	height int
}

func Open(path string) (*Target, error) {
	if !ValidTTY(path) {
		return nil, errors.New("not a terminal of this user")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}
	return &Target{f: f}, nil
}

func (t *Target) columns() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(t.f.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, err
	}
	return int(ws.Col), nil
}

// Draw paints pre-rendered rows in the top-right corner. The cursor position
// and text attributes are saved and restored around it, in a single write, so
// the shell's own display is left as it was.
func (t *Target) Draw(rows []string, width int) error {
	cols, err := t.columns()
	if err != nil {
		return err
	}
	if cols < width+8 {
		return t.Erase()
	}
	x := cols - width // one column is left free at the right edge
	var b strings.Builder
	b.WriteString("\x1b7")
	if t.drawn && t.lastX != x {
		t.blank(&b)
	}
	for i, r := range rows {
		fmt.Fprintf(&b, "\x1b[%d;%dH%s", i+1, x, r)
	}
	b.WriteString("\x1b8")
	t.drawn, t.lastX, t.width, t.height = true, x, width, len(rows)
	_, err = t.f.WriteString(b.String())
	return err
}

func (t *Target) blank(b *strings.Builder) {
	for i := 0; i < t.height; i++ {
		fmt.Fprintf(b, "\x1b[%d;%dH\x1b[0m%s", i+1, t.lastX, strings.Repeat(" ", t.width))
	}
}

// Erase clears the corner. What was under the creature is gone, so this leaves
// blank cells behind.
func (t *Target) Erase() error {
	if !t.drawn {
		return nil
	}
	var b strings.Builder
	b.WriteString("\x1b7")
	t.blank(&b)
	b.WriteString("\x1b8")
	t.drawn = false
	_, err := t.f.WriteString(b.String())
	return err
}

func (t *Target) Close() {
	t.Erase()
	t.f.Close()
}
