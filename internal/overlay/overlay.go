package overlay

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/bkodzo/po/internal/kitty"
	"github.com/bkodzo/po/internal/sprite"
)

// Mode is how the creature is drawn in the corner.
type Mode string

const (
	// Text draws block characters into the terminal's text. It works anywhere,
	// but the characters are copied along with any text selected near them.
	Text Mode = "text"
	// Image draws a real image on top of the terminal (Kitty graphics), so the
	// text underneath is untouched and nothing extra is copied.
	Image Mode = "image"
)

const (
	imagePlacement = 1
	imageZ         = 1 // above the text
	firstImageID   = 9100
	maxStored      = 96
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
	mode   Mode
	drawn  bool
	lastX  int
	width  int
	height int

	// Image mode: pictures already stored in the terminal, and which one is
	// showing now.
	stored map[string]int
	nextID int
	placed int
}

func Open(path string, mode Mode) (*Target, error) {
	if !ValidTTY(path) {
		return nil, errors.New("not a terminal of this user")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}
	return &Target{f: f, mode: mode, stored: map[string]int{}, nextID: firstImageID}, nil
}

func (t *Target) Mode() Mode { return t.mode }

func (t *Target) winsize() (*unix.Winsize, error) {
	return unix.IoctlGetWinsize(int(t.f.Fd()), unix.TIOCGWINSZ)
}

func (t *Target) columns() (int, error) {
	ws, err := t.winsize()
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
	if t.mode == Image {
		if t.placed == 0 {
			return nil
		}
		id := t.placed
		t.placed = 0
		_, err := t.f.WriteString(kitty.Remove(id, imagePlacement))
		return err
	}
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
	t.forget()
	t.f.Close()
}

// forget frees every picture this target stored in the terminal.
func (t *Target) forget() {
	if t.mode != Image || len(t.stored) == 0 {
		return
	}
	var b strings.Builder
	for _, id := range t.stored {
		b.WriteString(kitty.Free(id))
	}
	t.stored = map[string]int{}
	t.placed = 0
	t.f.WriteString(b.String())
}

// Refresh drops what was stored in the terminal, so the next draw sends it
// afresh. It is done at each new prompt, because a terminal may forget images
// when the screen is cleared, and a picture that is no longer there would leave
// the pet missing.
func (t *Target) Refresh() { t.forget() }

// DrawImage shows a frame as a real image in the top-right corner. key says
// which picture it is, so each one is sent to the terminal only once. The image
// is made at the terminal's own cell size, so it is shown sharp at exactly that
// size with no smoothing.
func (t *Target) DrawImage(key string, c *sprite.Canvas, pal sprite.Palette, cols, rows int) error {
	ws, err := t.winsize()
	if err != nil {
		return err
	}
	if int(ws.Col) < cols+8 {
		return t.Erase()
	}
	cellW, cellH := 8, 16 // used only if the terminal does not report its pixel size
	if ws.Col > 0 && ws.Xpixel > 0 && ws.Row > 0 && ws.Ypixel > 0 {
		cellW, cellH = int(ws.Xpixel)/int(ws.Col), int(ws.Ypixel)/int(ws.Row)
	}
	x := int(ws.Col) - cols // one column is left free at the right edge

	var out strings.Builder
	stored := fmt.Sprintf("%s@%dx%d", key, cellW, cellH)
	id, ok := t.stored[stored]
	if !ok {
		if len(t.stored) >= maxStored {
			t.forget()
		}
		id = t.nextID
		t.nextID++
		rgba, w, h := kitty.RGBA(c, pal, cellW*cols/c.W, cellH*rows/c.H)
		out.WriteString(kitty.Transmit(id, rgba, w, h))
		t.stored[stored] = id
	}
	if id == t.placed && x == t.lastX {
		return nil // already showing exactly this
	}
	out.WriteString("\x1b7")
	if t.placed != 0 {
		out.WriteString(kitty.Remove(t.placed, imagePlacement))
	}
	fmt.Fprintf(&out, "\x1b[1;%dH", x)
	out.WriteString(kitty.Place(id, imagePlacement, cols, rows, imageZ))
	out.WriteString("\x1b8")
	t.placed, t.lastX = id, x
	_, err = t.f.WriteString(out.String())
	return err
}
