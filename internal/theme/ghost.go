package theme

import (
	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/sprite"
)

// stampGhost is the round head of the default body with no feet, and a hem
// scalloped into three lobes. It floats a little above its shadow.
func stampGhost(c *sprite.Canvas, ox, oy int) {
	last := len(halfWidths) - 1
	for y, hw := range halfWidths {
		l, r := 8-hw, 8+hw-1
		for x := l; x <= r; x++ {
			ch := 'b'
			switch {
			case y == 0 || y == last || x == l || x == r:
				ch = 'o'
			case y == 12:
				ch = 's'
			case (y == 1 && (x == 5 || x == 6)) || (y == 2 && (x == 4 || x == 5)) || (y == 3 && x == 3):
				ch = 'l'
			}
			c.Set(ox+x, oy+y, ch)
		}
	}
	// Cut two notches out of the bottom edge so it has three lobes, and outline
	// the notches.
	for _, x := range []int{5, 6, 9, 10} {
		c.Set(ox+x, oy+12, 'o')
		c.Set(ox+x, oy+last, sprite.Clear)
	}
}

// Ghost is a floating ghost with spooky puns.
var Ghost = Theme{
	Name: "ghost",
	Body: stampGhost,
	Palette: sprite.Palette{
		'o': sprite.RGB(70, 80, 120),
		'b': sprite.RGB(235, 242, 255),
		'l': sprite.RGB(255, 255, 255),
		's': sprite.RGB(190, 204, 236),
		'f': sprite.RGB(190, 204, 236),
		'w': sprite.RGB(255, 255, 255),
		'k': sprite.RGB(40, 36, 60),
		'm': sprite.RGB(120, 70, 110),
		'c': sprite.RGB(250, 170, 200),
		'h': sprite.RGB(90, 90, 110),
		'g': sprite.RGB(225, 232, 240),
		'z': sprite.RGB(150, 190, 255),
		'y': sprite.RGB(255, 230, 90),
	},
	Tints: map[mood.Mood]Tint{
		mood.Frustrated:  {Color: sprite.RGB(235, 70, 60), Strength: 0.35},
		mood.Celebrating: {Color: sprite.RGB(255, 225, 120), Strength: 0.5},
		mood.Tired:       {Color: sprite.RGB(150, 150, 210), Strength: 0.35},
		mood.Focused:     {Color: sprite.RGB(110, 170, 235), Strength: 0.3},
	},
	Lines: map[mood.Mood][]string{
		mood.Frustrated:  {"Boo-hoo. That build is haunted.", "I've seen cleaner crime scenes.", "Who you gonna call? Not this compiler."},
		mood.Focused:     {"Haunting the codebase quietly.", "Floating through the diff.", "Spooky good focus."},
		mood.Celebrating: {"Boo-yah! Green tests!", "That was ghostly good.", "A truly ecto-plasmic win."},
		mood.Tired:       {"Dead tired.", "Time to rest in peace.", "Back to my coffin soon."},
		mood.Calm:        {"Just drifting.", "All quiet on the ghost front.", "Nothing but cobwebs."},
	},
	Unsure: []string{"Did I see something?", "Was that the ghost of an error?"},
}
