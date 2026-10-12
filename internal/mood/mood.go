package mood

type Mood string

const (
	Frustrated  Mood = "frustrated"
	Focused     Mood = "focused"
	Celebrating Mood = "celebrating"
	Tired       Mood = "tired"
	Calm        Mood = "calm"
)

var All = []Mood{Frustrated, Focused, Celebrating, Tired, Calm}

type Event struct {
	Kind string
	Text string

	// Set for Kind "command": the redacted command, how it ended, how many
	// commands in a row failed before it, and the local hour (0-23).
	Cmd        string
	Exit       int
	Millis     int64
	FailStreak int
	Hour       int
}

type Reading struct {
	Scores map[Mood]float64
}

func (r Reading) Top() (Mood, float64) {
	best, score := Calm, 0.0
	for _, m := range All {
		if s := r.Scores[m]; s > score {
			best, score = m, s
		}
	}
	return best, score
}

// Reader turns an event into a mood reading. Scores sum to 1.
type Reader interface {
	Read(Event) (Reading, error)
}
