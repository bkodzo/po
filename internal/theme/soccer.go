package theme

import (
	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/sprite"
)

var Soccer = Theme{
	Name: "soccer",
	Palette: sprite.Palette{
		'o': sprite.RGB(60, 40, 50),
		'b': sprite.RGB(245, 160, 70),
		'l': sprite.RGB(255, 205, 130),
		's': sprite.RGB(205, 120, 50),
		'f': sprite.RGB(190, 105, 45),
		'w': sprite.RGB(255, 255, 255),
		'k': sprite.RGB(40, 30, 40),
		'm': sprite.RGB(150, 40, 50),
		'c': sprite.RGB(255, 130, 130),
		'h': sprite.RGB(90, 90, 100),
		'g': sprite.RGB(225, 232, 240),
		'z': sprite.RGB(150, 190, 255),
		'y': sprite.RGB(255, 230, 90),
	},
	Tints: map[mood.Mood]Tint{
		mood.Frustrated:  {Color: sprite.RGB(235, 70, 60), Strength: 0.45},
		mood.Celebrating: {Color: sprite.RGB(255, 205, 60), Strength: 0.45},
		// Tired is a dusty lilac and focused a cool hint, because orange pushed
		// toward blue turns a muddy grey.
		mood.Tired:   {Color: sprite.RGB(150, 130, 200), Strength: 0.40},
		mood.Focused: {Color: sprite.RGB(90, 170, 220), Strength: 0.22},
	},
	Lines: map[mood.Mood][]string{
		mood.Frustrated:  {"That was a red card build.", "Bad tackle by the compiler.", "We go again."},
		mood.Focused:     {"Midfield is under control.", "Building from the back.", "Nice, patient play."},
		mood.Celebrating: {"GOAL! Tests are green!", "Top bins!", "Take a bow, striker."},
		mood.Tired:       {"Is it full time yet?", "Extra time again?", "Subbing myself off."},
		mood.Calm:        {"Quiet half so far.", "Keeping the shape.", "Nothing to report, gaffer."},
	},
	Unsure: []string{"Wait, was that offside?", "VAR is checking that one..."},
}
