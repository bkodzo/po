package daemon

import (
	"math/rand"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bkodzo/po/internal/config"
	"github.com/bkodzo/po/internal/event"
	"github.com/bkodzo/po/internal/idle"
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

// State is what a body (a window that draws the pet) needs to know, streamed to
// subscribers whenever it changes.
type State struct {
	V int `json:"v"`
	// Mood is what to show now: a reaction to the latest command while it is
	// fresh, otherwise the longer-term mood.
	Mood string `json:"mood"`
	// Resting is the longer-term mood.
	Resting string `json:"resting"`
	// Line is the latest thing the pet said, and LineSeq goes up each time it
	// says something new, so a body can tell a repeat from a new line.
	Line    string `json:"line,omitempty"`
	LineSeq int    `json:"line_seq"`
	Theme   string `json:"theme"`
	// Activity is an idle animation to play ("look", "stretch", "sleep", "wake"),
	// or empty. ActivitySeq goes up each time one starts, so a body can tell a
	// new one from the one it is already playing.
	Activity    string `json:"activity,omitempty"`
	ActivitySeq int    `json:"activity_seq"`
}

// StateVersion is bumped when State changes in a way a body would notice.
const StateVersion = 1

type subscriber struct {
	req  ipc.Request
	ch   chan State
	dead atomic.Bool
}

// Status is what `po status` reports.
type Status struct {
	Overlay       bool        `json:"overlay"`
	Mood          string      `json:"mood"`
	Reader        string      `json:"reader"`
	Model         *ModelState `json:"model,omitempty"`
	AnsweredFast  int         `json:"answered_fast"`
	AnsweredModel int         `json:"answered_model"`
	ModelFailed   int         `json:"model_failed"`
	Events        int         `json:"events"`
	FailStreak    int         `json:"fail_streak"`
	Terminals     int         `json:"terminals"`
	Subscribers   int         `json:"subscribers"`
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
	line       string
	lineSeq    int

	tick    int
	targets map[string]*overlay.Target
	queue   chan mood.Event

	subs      map[*subscriber]struct{}
	lastState State

	overlayOn bool

	// Idle life. Only the main loop touches these.
	sched         *idle.Scheduler
	lastEvent     time.Time
	activity      string
	activityLoop  bool
	activityUntil time.Time
	activitySeq   int
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
		subs:    map[*subscriber]struct{}{},

		overlayOn: st.Cfg.Overlay != "off",
	}
	now := time.Now()
	d.lastEvent = now
	d.sched = idle.New(idleConfig(st.Cfg), rand.New(rand.NewSource(now.UnixNano())), now)
	d.exportTheme()
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
			if req.Op != "subscribe" {
				req.Ack()
			}
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
	case "subscribe":
		d.subscribe(req)
		return false
	case "reload":
		d.reload()
		return false
	case "idle":
		d.playActivity(req.Name)
		return false
	case "theme":
		// A window asked for another theme, so remember the choice too.
		if d.setTheme(req.Name) {
			if cfg, err := config.Load(); err == nil {
				cfg.Theme = req.Name
				_ = cfg.Save()
			}
		}
		return false
	}
	if req.Cmd != nil {
		// Any command, even one too dull to react to, means someone is there.
		d.userActive()
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
		Overlay:       d.overlayOn,
		Mood:          m,
		Reader:        d.stack.Cfg.Reader,
		Events:        d.events,
		FailStreak:    d.failStreak,
		Terminals:     len(d.targets),
		Subscribers:   len(d.subs),
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
				d.line = d.th.Say(top)
				d.lineSeq++
			}
			d.pet.Apply(reading)
		}
		_ = d.pet.Save()
		d.mu.Unlock()
	}
}

// reload re-reads the settings that can change while Po runs. Turning the
// corner pet off clears it from every terminal straight away.
func (d *Daemon) reload() {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	d.overlayOn = cfg.Overlay != "off"
	d.setTheme(cfg.Theme)
	d.sched.SetConfig(idleConfig(cfg))
	if cfg.Idle == "off" {
		d.setActivity("")
	}
	if !d.overlayOn {
		for tty := range d.targets {
			d.hide(tty)
		}
	}
}

func (d *Daemon) show(tty string) {
	if !d.overlayOn {
		return
	}
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

// idleConfig turns the settings into the scheduler's.
func idleConfig(cfg config.Config) idle.Config {
	c := idle.DefaultConfig()
	c.On = cfg.Idle != "off"
	if cfg.SleepMinutes > 0 {
		c.SleepAfter = time.Duration(cfg.SleepMinutes) * time.Minute
	}
	return c
}

// userActive notes that someone is at the keyboard. A sleeping pet wakes with a
// start, and any other idle animation is cut short.
func (d *Daemon) userActive() {
	d.lastEvent = time.Now()
	switch d.activity {
	case "":
	case "sleep":
		d.setActivity("wake")
	default:
		d.setActivity("")
	}
}

// setActivity starts an idle animation, or clears it if name is empty.
func (d *Daemon) setActivity(name string) {
	d.activity = name
	d.activityLoop = false
	if name == "" {
		return
	}
	info, ok := theme.ActivityOf(name)
	if !ok {
		d.activity = ""
		return
	}
	d.activitySeq++
	d.activityLoop = info.Loop
	d.activityUntil = time.Now().Add(time.Duration(info.Ticks) * time.Second / theme.Fps)
}

// playActivity plays an idle animation on request, for `po idle <name>`.
func (d *Daemon) playActivity(name string) {
	if _, ok := theme.ActivityOf(name); ok {
		d.setActivity(name)
	}
}

// idleTick ends a finished animation and starts a new one when it is time.
func (d *Daemon) idleTick() {
	now := time.Now()
	if d.activity != "" && !d.activityLoop && now.After(d.activityUntil) {
		d.setActivity("")
	}
	if d.activity != "" {
		return
	}
	d.mu.Lock()
	resting := d.pet.Mood
	busy := now.Before(d.flashUntil)
	d.mu.Unlock()
	if name := d.sched.Pick(now, d.lastEvent, resting, busy); name != "" {
		d.setActivity(name)
	}
}

func (d *Daemon) redraw() {
	d.tick++
	d.idleTick()
	d.mu.Lock()
	d.pet.Decay(tickEvery)
	d.mu.Unlock()
	for tty := range d.targets {
		d.draw(tty)
	}
	d.broadcast()
}

// current is the state as of now.
func (d *Daemon) current() State {
	d.mu.Lock()
	defer d.mu.Unlock()
	shown := d.pet.Mood
	if time.Now().Before(d.flashUntil) {
		shown = d.flash
	}
	// Idle animations only show while the creature is at rest.
	activity := d.activity
	if time.Now().Before(d.flashUntil) || (d.pet.Mood != mood.Calm && d.pet.Mood != mood.Focused) {
		activity = ""
	}
	return State{
		V: StateVersion, Mood: string(shown), Resting: string(d.pet.Mood),
		Line: d.line, LineSeq: d.lineSeq, Theme: d.th.Name,
		Activity: activity, ActivitySeq: d.activitySeq,
	}
}

// subscribe starts streaming state to a body. The first reply is the current
// state, so a body that connects late is correct straight away.
func (d *Daemon) subscribe(req ipc.Request) {
	sub := &subscriber{req: req, ch: make(chan State, 4)}
	req.Reply(d.current())
	d.subs[sub] = struct{}{}
	go func() {
		for st := range sub.ch {
			if req.Write(st) != nil {
				break
			}
		}
		sub.dead.Store(true)
	}()
}

// broadcast sends the state to every subscriber if it has changed. A slow
// subscriber never holds up the pet: if its queue is full, its oldest update is
// dropped.
func (d *Daemon) broadcast() {
	for sub := range d.subs {
		if sub.dead.Load() {
			close(sub.ch)
			delete(d.subs, sub)
		}
	}
	st := d.current()
	if st == d.lastState {
		return
	}
	d.lastState = st
	for sub := range d.subs {
		for {
			select {
			case sub.ch <- st:
			default:
				select {
				case <-sub.ch:
				default:
				}
				continue
			}
			break
		}
	}
}

// exportTheme writes the theme where a window can find it, next to the socket.
func (d *Daemon) exportTheme() {
	sock, err := ipc.Path()
	if err != nil {
		return
	}
	_ = d.currentTheme().WriteFile(filepath.Join(filepath.Dir(sock), "theme.json"))
}

// currentTheme is the theme in use now. It can change while Po runs.
func (d *Daemon) currentTheme() theme.Theme {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.th
}

// setTheme switches to another theme and writes it out for the windows. It does
// nothing for a name that does not exist. The new name reaches the windows in
// the next state update, after the file is already written.
func (d *Daemon) setTheme(name string) bool {
	th, ok := theme.Get(name)
	if !ok {
		return false
	}
	d.mu.Lock()
	changed := d.th.Name != name
	d.th = th
	d.mu.Unlock()
	if changed {
		d.exportTheme()
	}
	return true
}

func (d *Daemon) draw(tty string) {
	d.mu.Lock()
	m := d.pet.Mood
	if time.Now().Before(d.flashUntil) {
		m = d.flash
	}
	d.mu.Unlock()

	canvas, pal := d.currentTheme().MiniFrame(m, d.tick)
	rows := strings.Split(canvas.Render(pal), "\n")
	if err := d.targets[tty].Draw(rows, theme.MiniW); err != nil {
		// The terminal is gone, so forget it.
		d.targets[tty].Close()
		delete(d.targets, tty)
	}
}

func (d *Daemon) shutdown() {
	d.stack.Close()
	for sub := range d.subs {
		close(sub.ch)
		delete(d.subs, sub)
	}
	for tty := range d.targets {
		d.hide(tty)
	}
	close(d.queue)
	d.mu.Lock()
	_ = d.pet.Save()
	d.mu.Unlock()
}
