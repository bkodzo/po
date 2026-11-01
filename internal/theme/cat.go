package theme

import (
	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/sprite"
)

// stampCat is the default body with a pair of pointed ears and a small nose.
// The ears stand two rows above the head, so they stay inside the canvas even
// at the top of a jump.
func stampCat(c *sprite.Canvas, ox, oy int) {
	stampBody(c, ox, oy)
	for _, side := range []int{0, 1} {
		at := func(x int) int { // the right ear mirrors the left across the head
			if side == 1 {
				return 15 - x
			}
			return x
		}
		// A solid triangle in the fur colour with a pink middle. An outline
		// would vanish against a dark terminal, so there is none.
		c.Set(ox+at(4), oy-2, 'b')
		c.Set(ox+at(3), oy-1, 'b')
		c.Set(ox+at(4), oy-1, 'e')
		c.Set(ox+at(5), oy-1, 'b')
	}
	c.Set(ox+7, oy+8, 'c')
	c.Set(ox+8, oy+8, 'c')
}

// Cat is a grey cat with cat puns.
var Cat = Theme{
	Name: "cat",
	Body: stampCat,
	Palette: sprite.Palette{
		'o': sprite.RGB(45, 45, 62),
		'b': sprite.RGB(168, 172, 188),
		'l': sprite.RGB(205, 210, 225),
		's': sprite.RGB(125, 130, 150),
		'f': sprite.RGB(110, 115, 135),
		'e': sprite.RGB(240, 150, 170),
		'w': sprite.RGB(255, 255, 255),
		'k': sprite.RGB(30, 30, 40),
		'm': sprite.RGB(140, 60, 70),
		'c': sprite.RGB(245, 150, 170),
		'h': sprite.RGB(90, 90, 100),
		'g': sprite.RGB(225, 232, 240),
		'z': sprite.RGB(150, 190, 255),
		'y': sprite.RGB(255, 230, 90),
	},
	Tints: map[mood.Mood]Tint{
		mood.Frustrated:  {Color: sprite.RGB(235, 80, 70), Strength: 0.4},
		mood.Celebrating: {Color: sprite.RGB(255, 215, 100), Strength: 0.4},
		mood.Tired:       {Color: sprite.RGB(140, 130, 200), Strength: 0.35},
		mood.Focused:     {Color: sprite.RGB(90, 170, 220), Strength: 0.25},
	},
	Lines: map[mood.Mood][]string{
		mood.Frustrated:  {"Hiss. That build failed.", "I'm knocking this off the table.", "Not purr-fect."},
		mood.Focused:     {"Stalking the bug.", "Pouncing on the diff.", "Eyes on the laser dot."},
		mood.Celebrating: {"Purr-fect! Tests pass.", "Paw-sitively brilliant!", "That was the cat's whiskers."},
		mood.Tired:       {"Nap time. The first of many.", "Need a catnap.", "Curling up now."},
		mood.Calm:        {"Purring softly.", "Just loafing.", "Nothing to chase."},
	},
	Milestones: map[string][]string{
		"record":     {"%d failures in a row. I would knock it off the table.", "A record %d failures. Hiss."},
		"relief":     {"Purr-fect! Green after %d failures.", "%d failures, then success. Paw-sitively good."},
		"late_run":   {"%d late nights in a row. Cats are nocturnal, you are not.", "Night %d running. Go and sleep, human."},
		"comeback":   {"Back after %d hours. I was napping anyway.", "%d hours away. I did not miss you. A little."},
		"busy":       {"%d commands today. Even I am tired.", "That is %d commands. Quite the day."},
		"first_push": {"First push of the day. Good human.", "Pushed already. Here, a head bump."},
	},
	Recaps: map[string][]string{
		"quiet":  {"A quiet week. Excellent napping.", "Hardly any commands. I approve."},
		"rough":  {"A rough week, but you landed on your feet.", "Many failures, yet you kept pouncing."},
		"steady": {"A steady week. Nicely loafed.", "Nothing dramatic. Good."},
		"great":  {"A purr-fect week.", "The cat's whiskers of a week."},
	},
	Unsure: []string{"Was that a mouse?", "Was that a laser dot?"},
}
