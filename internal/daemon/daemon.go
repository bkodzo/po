package daemon

import (
	"strings"
	"sync"
	"time"

	"github.com/bkodzo/po/internal/ipc"
	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/overlay"
	"github.com/bkodzo/po/internal/pet"
	"github.com/bkodzo/po/internal/theme"
)

const (
	tickEvery = time.Second / theme.MiniFps
	// maxQueued bounds how many events wait for the reader. Past this the
	// oldest are dropped, so a burst of commands never leaves the pet reacting
	// to old news.
	maxQueued = 3
	// flashFor is how long the creature shows its reaction to the latest
	// command before going back to its longer-term mood.
	flashFor = 3 * time.Second
)

// Daemon runs the pet with no window of its own. It draws a two-row creature
// in the corner of each terminal whose shell is waiting at a prompt.
type Daemon struct {
	th     theme.Theme
	reader mood.Reader

	mu         sync.Mutex // guards pet and flash
	pet        *pet.Pet
	flash      mood.Mood
	flashUntil time.Time

	tick    int
	targets map[string]*overlay.Target
	queue   chan mood.Event
}

// Run serves requests until a stop message arrives.
func Run(th theme.Theme, p *pet.Pet, r mood.Reader, l *ipc.Listener) {
	d := &Daemon{
		th:      th,
		reader:  r,
		pet:     p,
		targets: map[string]*overlay.Target{},
		queue:   make(chan mood.Event, maxQueued),
	}
	go d.worker()

	ticker := time.NewTicker(tickEvery)
	defer ticker.Stop()
	for {
		select {
		case req := <-l.C:
			stop := d.handle(req)
			req.Ack()
			if stop {
				d.shutdown()
				return
			}
		case <-ticker.C:
			d.redraw()
		}
	}
}

func (d *Daemon) handle(req ipc.Request) (stop bool) {
	switch req.Op {
	case "stop":
		return true
	case "hide":
		d.hide(req.TTY)
		return false
	}
	if req.Cmd != nil && !req.Cmd.Ignore() {
		d.submit(req.Cmd.ToEvent())
	}
	if req.TTY != "" {
		d.show(req.TTY)
	}
	return false
}

// submit queues an event, dropping the oldest if the queue is full.
func (d *Daemon) submit(e mood.Event) {
	for {
		select {
		case d.queue <- e:
			return
		default:
			select {
			case <-d.queue:
			default:
			}
		}
	}
}

// worker reads events one at a time, since the model copes poorly with
// parallel requests.
func (d *Daemon) worker() {
	for e := range d.queue {
		reading, err := d.reader.Read(e)
		d.mu.Lock()
		if err == nil {
			if top, conf := reading.Top(); conf >= d.pet.MinConfidence {
				d.flash, d.flashUntil = top, time.Now().Add(flashFor)
			}
			d.pet.Apply(reading)
		}
		_ = d.pet.Save()
		d.mu.Unlock()
	}
}

func (d *Daemon) show(tty string) {
	if _, ok := d.targets[tty]; !ok {
		t, err := overlay.Open(tty)
		if err != nil {
			return
		}
		d.targets[tty] = t
	}
	d.draw(tty)
}

func (d *Daemon) hide(tty string) {
	if t, ok := d.targets[tty]; ok {
		t.Close()
		delete(d.targets, tty)
	}
}

func (d *Daemon) redraw() {
	d.tick++
	d.mu.Lock()
	d.pet.Decay(tickEvery)
	d.mu.Unlock()
	for tty := range d.targets {
		d.draw(tty)
	}
}

func (d *Daemon) draw(tty string) {
	d.mu.Lock()
	m := d.pet.Mood
	if time.Now().Before(d.flashUntil) {
		m = d.flash
	}
	d.mu.Unlock()

	canvas, pal := d.th.MiniFrame(m, d.tick)
	rows := strings.Split(canvas.Render(pal), "\n")
	if err := d.targets[tty].Draw(rows, theme.MiniW); err != nil {
		// The terminal is gone, so forget it.
		d.targets[tty].Close()
		delete(d.targets, tty)
	}
}

func (d *Daemon) shutdown() {
	for tty := range d.targets {
		d.hide(tty)
	}
	close(d.queue)
	d.mu.Lock()
	_ = d.pet.Save()
	d.mu.Unlock()
}
