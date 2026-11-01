package theme

import (
	"sort"

	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/sprite"
)

// ActivityInfo describes an idle animation: how many ticks it lasts, and
// whether it repeats until told to stop (sleeping) or plays once.
type ActivityInfo struct {
	Ticks int
	Loop  bool
}

// activities are the little things the creature does when nothing is happening.
var activities = map[string]ActivityInfo{
	"look":    {Ticks: 28},
	"stretch": {Ticks: 24},
	"sleep":   {Ticks: 32, Loop: true},
	"wake":    {Ticks: 10},
	// peek is a long, curious glance. The window slides partly off a screen edge
	// while it plays, if the creature is next to one.
	"peek": {Ticks: 48},
}

// ActivityNames lists every idle animation, in a stable order.
func ActivityNames() []string {
	names := make([]string, 0, len(activities))
	for n := range activities {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ActivityOf returns the details of an idle animation.
func ActivityOf(name string) (ActivityInfo, bool) {
	a, ok := activities[name]
	return a, ok
}

// ActivityFrame draws one tick of an idle animation. Activities use the calm
// palette: they happen while the creature is at rest.
func (t Theme) ActivityFrame(name string, tick int) (*sprite.Canvas, sprite.Palette) {
	info, ok := activities[name]
	if !ok {
		return t.Frame(mood.Calm, tick)
	}
	if info.Loop {
		tick %= info.Ticks
	} else if tick >= info.Ticks {
		tick = info.Ticks - 1 // a one-off holds its last picture
	}
	p := activityPose(name, tick)
	c := sprite.NewCanvas(CanvasW, CanvasH)
	shadow(c, p.dy)
	t.stamp(c, bodyX+p.dx, bodyY+p.dy)
	face(c, bodyX+p.dx, bodyY+p.dy, p)
	activityExtras(c, name, tick)
	return c, t.paletteFor(mood.Calm)
}

func activityPose(name string, t int) pose {
	switch name {
	case "look":
		// Glance left, then right, then back to the middle.
		gaze := 0
		switch {
		case t >= 4 && t < 10:
			gaze = -1
		case t >= 14 && t < 22:
			gaze = 1
		}
		return pose{eyes: "open", gaze: gaze, mouth: "smile", arms: "down", cheeks: true}

	case "peek":
		// Glances in turn toward each side, with a small curious mouth, and
		// keeps its eyes on the middle at the start and end.
		gaze := 0
		switch {
		case t >= 8 && t < 16, t >= 28 && t < 34:
			gaze = -1
		case t >= 18 && t < 26, t >= 36 && t < 42:
			gaze = 1
		}
		return pose{eyes: "open", gaze: gaze, mouth: "flat", arms: "down"}

	case "stretch":
		p := pose{eyes: "closed", mouth: "yawn", arms: "up", cheeks: true}
		switch {
		case t < 4:
			p.dy = -t / 2 // rising onto toes
		case t < 14:
			p.dy = -2
			if t%2 == 0 {
				p.dx = 0
			}
		case t < 20:
			p.dy = -1
			p.arms = "down"
			p.mouth = "flat"
			if t >= 17 {
				p.eyes = "open"
			}
		default:
			p = pose{eyes: "open", mouth: "smile", arms: "down", cheeks: true}
		}
		return p

	case "sleep":
		p := pose{eyes: "closed", mouth: "flat", arms: "droop"}
		if (t/4)%2 == 1 {
			p.dy = 1 // breathing
		}
		return p

	case "wake":
		p := pose{eyes: "closed", mouth: "flat", arms: "droop"}
		switch {
		case t < 2:
		case t < 5:
			p = pose{eyes: "open", mouth: "yawn", arms: "up", dy: -2}
		default:
			p = pose{eyes: "open", mouth: "smile", arms: "down", cheeks: true}
		}
		return p
	}
	return pose{eyes: "open", mouth: "smile", arms: "down", cheeks: true}
}

func activityExtras(c *sprite.Canvas, name string, t int) {
	cx := bodyX + 8
	switch name {
	case "sleep":
		// A big Z and, half a cycle behind it, a smaller one, both drifting up.
		for i, phase := range []int{0, 16} {
			age := (t + phase) % 32
			if age > 22 {
				continue
			}
			if i == 0 {
				y := 1 - age/12
				if y < 0 {
					y = 0
				}
				drawZ(c, cx+5, y)
				continue
			}
			y := 2 - age/10
			if y < 0 {
				y = 0
			}
			drawShape(c, 1, y, smallZ)
		}
	case "wake":
		// A yellow exclamation mark, as if startled awake.
		if t >= 2 && t < 8 {
			for y := 0; y < 3; y++ {
				c.Set(cx, 1+y, 'y')
			}
			c.Set(cx, 5, 'y')
		}
	}
}
