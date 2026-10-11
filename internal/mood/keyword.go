package mood

import "strings"

var keywords = map[Mood][]string{
	Frustrated:  {"ugh", "fail", "error", "broken", "wtf", "damn", "panic", "again", "why"},
	Focused:     {"building", "working", "refactor", "wip", "implement", "test"},
	Celebrating: {"pass", "passed", "works", "done", "shipped", "merged", "finally", "yes"},
	Tired:       {"tired", "sleep", "late", "zzz", "later", "break"},
}

// Keyword is a rule-based stand-in for a model-backed reader.
type Keyword struct{}

func (Keyword) Read(e Event) (Reading, error) {
	text := strings.ToLower(e.Text)
	scores := make(map[Mood]float64, len(All))
	total := 0.0
	for m, words := range keywords {
		for _, w := range words {
			if strings.Contains(text, w) {
				scores[m]++
				total++
			}
		}
	}
	if total == 0 {
		for _, m := range All {
			scores[m] = 1 / float64(len(All))
		}
		return Reading{Scores: scores}, nil
	}
	for m := range scores {
		scores[m] /= total
	}
	return Reading{Scores: scores}, nil
}
