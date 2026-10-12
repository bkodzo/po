package daemon

import (
	"strings"
	"sync"
	"time"

	"github.com/bkodzo/po/internal/event"
	"github.com/bkodzo/po/internal/ipc"
	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/overlay"
	"github.com/bkodzo/po/internal/pet"
	"github.com/bkodzo/po/internal/stack"
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
	// streakResets is how long without a command ends a run of failures.
	streakResets = 30 * time.Minute
)

// Status is what `po status` reports.
type Status struct {
	Mood          string      `json:"mood"`
	Reader        string      `json:"reader"`
	Model         *ModelState `json:"model,omitempty"`
	AnsweredFast  int         `json:"answered_fast"`
	AnsweredModel int         `json:"answered_model"`
	ModelFailed   int         `json:"model_failed"`
	Events        int         `json:"events"`
	FailStreak    int         `json:"fail_streak"`
	Terminals     int         `json:"terminals"`
	UptimeSeconds int         `json:"uptime_seconds"`
}

// ModelState describes the model Po may ask.
type ModelState struct {
	URL       string `json:"url"`
	Model     string `json:"model"`
	Local     bool   `json:"local"`
	Reachable bool   `json:"reachable"`
	// Process describes the server when Po manages it.
	Process string `json:"process,omitempty"`
	Problem string `json:"problem,omitempty"`
}

// Daemon runs the pet with no window of its own. It draws a two-row creature
// in the corner of each terminal whose shell is waiting at a prompt.
type Daemon struct {
	th     theme.Theme
	stack  *stack.Stack
	reader mood.Reader

	started    time.Time
	events     int
	failStreak int
	lastCmd    time.Time

	mu         sync.Mutex // guards pet and flash
	pet        *pet.Pet
	flash      mood.Mood
	flashUntil time.Time

	tick    int
	targets map[string]*overlay.Target
	queue   chan mood.Event
}

// Run serves requests until a stop message arrives.
func Run(th theme.Theme, p *pet.Pet, st *stack.Stack, l *ipc.Listener) {
	d := &Daemon{
		th:      th,
		stack:   st,
		reader:  st.Reader,
		started: time.Now(),
		pet:     p,
		targets: map[string]*overlay.Target{},
		queue:   make(chan mood.Event, maxQueued),
	}
	go d.worker()
	stopWatch := make(chan struct{})
	defer close(stopWatch)
	if st.Managed != nil {
		go st.Managed.Watch(stopWatch)
	}

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
	case "status":
		req.Reply(d.status())
		return false
	}
	if req.Cmd != nil && !req.Cmd.Ignore() {
		d.submit(d.eventFor(*req.Cmd))
	}
	if req.TTY != "" {
		d.show(req.TTY)
	}
	return false
}

// eventFor adds what the daemon knows about the run of commands so far: how
// many failed in a row, and the hour.
func (d *Daemon) eventFor(c event.Command) mood.Event {
	now := time.Now()
	if now.Sub(d.lastCmd) > streakResets {
		d.failStreak = 0
	}
	d.lastCmd = now
	d.events++
	e := c.ToEventAt(now.Hour(), d.failStreak)
	switch {
	case c.Exit == 0:
		d.failStreak = 0
	case c.Exit < 128: // a Ctrl-C or kill is not a failure of the work
		d.failStreak++
	}
	return e
}

func (d *Daemon) status() Status {
	d.mu.Lock()
	m := string(d.pet.Mood)
	d.mu.Unlock()
	st := Status{
		Mood:          m,
		Reader:        d.stack.Cfg.Reader,
		Events:        d.events,
		FailStreak:    d.failStreak,
		Terminals:     len(d.targets),
		UptimeSeconds: int(time.Since(d.started).Seconds()),
	}
	if d.stack.Routed != nil {
		rs := d.stack.Routed.Stats()
		st.AnsweredFast, st.AnsweredModel, st.ModelFailed = rs.Fast, rs.Slow, rs.SlowFailed
	}
	if d.stack.UsesModel() {
		st.Model = &ModelState{
			URL:       d.stack.Cfg.URL,
			Model:     d.stack.Cfg.Model,
			Local:     stack.IsLocal(d.stack.Cfg.URL),
			Reachable: stack.Reachable(d.stack.Cfg.URL),
		}
		if d.stack.Managed != nil {
			st.Model.Process = d.stack.Managed.State()
		}
		if d.stack.ManagedErr != nil {
			st.Model.Problem = d.stack.ManagedErr.Error()
		}
	}
	return st
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
	d.stack.Close()
	for tty := range d.targets {
		d.hide(tty)
	}
	close(d.queue)
	d.mu.Lock()
	_ = d.pet.Save()
	d.mu.Unlock()
}
