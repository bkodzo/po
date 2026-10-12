package mood

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Decision reads mood from any server that speaks the /v1/systemone decision
// format, such as a local llama-server running a decision model.
type Decision struct {
	URL    string
	Model  string
	Key    string
	Client *http.Client
	// TextOnly sends command events as a plain sentence, as the first version
	// did. It exists so the two ways of asking can be compared.
	TextOnly bool
}

func NewDecision(url, model string) *Decision {
	if url == "" {
		url = "http://127.0.0.1:8080"
	}
	if model == "" {
		model = "clef-flash"
	}
	return &Decision{
		URL:    url,
		Model:  model,
		Key:    os.Getenv("PO_API_KEY"),
		Client: &http.Client{Timeout: 20 * time.Second},
	}
}

type decisionAnswer struct {
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

type decisionResponse struct {
	Answers map[string]decisionAnswer `json:"answers"`
}

func criteria() map[string]string {
	c := make(map[string]string, len(All))
	for _, m := range All {
		c[string(m)] = descriptions[m]
	}
	return c
}

// commandCriteria describe the moods for a finished shell command, and point at
// the fields of the structured state that matter for each.
func commandCriteria() map[string]string {
	return map[string]string{
		string(Frustrated):  "the command failed, especially again and again (see failures_in_a_row_before), or the command was not found",
		string(Celebrating): "something notable worked: a push, merge, deploy or release, tests or a build passing, or the first success after several failures",
		string(Focused):     "steady, productive work: editing, building, committing, reading code, searching",
		string(Tired):       "time_of_day is the small hours or late night, so the person is worn out whatever the command was, even if it succeeded",
		string(Calm):        "routine, low-stakes commands with nothing notable: listing, status checks, navigation, quick lookups, or a command the user stopped on purpose",
	}
}

func outcome(e Event) string {
	switch {
	case e.Exit == 0:
		return "succeeded"
	case e.Exit == 127:
		return "failed: command not found"
	case e.Exit == 130 || e.Exit == 131 || e.Exit == 143:
		return "stopped by the user on purpose (not a failure)"
	}
	return "failed"
}

func timeOfDay(h int) string {
	switch {
	case h >= 0 && h < 5:
		return "the small hours of the night (after midnight)"
	case h >= 23:
		return "late at night"
	case h < 12:
		return "morning"
	case h < 18:
		return "afternoon"
	}
	return "evening"
}

// commandState is the event as structured fields, which suits the decision
// model better than a sentence.
func commandState(e Event) map[string]any {
	return map[string]any{
		"command":                  e.Cmd,
		"exit_code":                e.Exit,
		"outcome":                  outcome(e),
		"seconds_taken":            e.Millis / 1000,
		"failures_in_a_row_before": e.FailStreak,
		"local_hour":               e.Hour,
		"time_of_day":              timeOfDay(e.Hour),
	}
}

func (d *Decision) Read(e Event) (Reading, error) {
	var state any = fmt.Sprintf("%s: %s", e.Kind, e.Text)
	instructions := "What mood is the developer in, judging by this terminal activity?"
	crit := criteria()
	if e.Kind == "command" && !d.TextOnly {
		state = commandState(e)
		instructions = "A developer just ran this shell command. What mood is it likely to leave them in?"
		crit = commandCriteria()
	}
	body, err := json.Marshal(map[string]any{
		"model": d.Model,
		"state": state,
		"questions": map[string]any{
			"mood": map[string]any{
				"type":         "choice",
				"instructions": instructions,
				"criteria":     crit,
			},
		},
	})
	if err != nil {
		return Reading{}, err
	}
	req, err := http.NewRequest(http.MethodPost, d.URL+"/v1/systemone", bytes.NewReader(body))
	if err != nil {
		return Reading{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if d.Key != "" {
		req.Header.Set("Authorization", "Bearer "+d.Key)
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return Reading{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Reading{}, fmt.Errorf("decision server returned %s", resp.Status)
	}
	var out decisionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Reading{}, err
	}
	return parseDecision(out)
}

func parseDecision(out decisionResponse) (Reading, error) {
	ans, ok := out.Answers["mood"]
	if !ok || len(ans.Probabilities) == 0 {
		return Reading{}, fmt.Errorf("decision server returned no mood answer")
	}
	scores := make(map[Mood]float64, len(All))
	total := 0.0
	for _, m := range All {
		p := ans.Probabilities[string(m)]
		scores[m] = p
		total += p
	}
	if total == 0 {
		return Reading{}, fmt.Errorf("decision server returned no usable probabilities")
	}
	for m := range scores {
		scores[m] /= total
	}
	return Reading{Scores: scores}, nil
}
