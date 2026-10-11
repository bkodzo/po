package theme

import "github.com/bkodzo/critter/internal/mood"

var Soccer = Theme{
	Name: "soccer",
	Art: map[mood.Mood]string{
		mood.Frustrated:  " (>_<)\n  /|\\ \n  / \\  o",
		mood.Focused:     " (o_o)\n  /|\\ \n  / \\ o",
		mood.Celebrating: "\\(^o^)/\n   |   \n  / \\ o",
		mood.Tired:       " (-_-)\n  /|\\ \n  / \\ zz",
		mood.Calm:        " (^_^)\n  /|\\ \n  / \\  o",
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
