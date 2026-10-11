package theme

import (
	"github.com/bkodzo/critter/internal/mood"
	"github.com/bkodzo/critter/internal/sprite"
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
	Tints: map[mood.Mood]sprite.Color{
		mood.Frustrated:  sprite.RGB(235, 70, 60),
		mood.Focused:     sprite.RGB(90, 170, 220),
		mood.Celebrating: sprite.RGB(255, 205, 60),
		mood.Tired:       sprite.RGB(110, 120, 190),
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
