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

func (d *Decision) Read(e Event) (Reading, error) {
	body, err := json.Marshal(map[string]any{
		"model": d.Model,
		"state": fmt.Sprintf("%s: %s", e.Kind, e.Text),
		"questions": map[string]any{
			"mood": map[string]any{
				"type":         "choice",
				"instructions": "What mood is the developer in, judging by this terminal activity?",
				"criteria":     criteria(),
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
