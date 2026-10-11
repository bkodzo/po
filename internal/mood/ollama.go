package mood

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

const DefaultOllamaModel = "hf.co/bartowski/Cloudflare_clef-flash-GGUF:Q4_K_M"

// Ollama reads mood from a local model. It asks for a single digit and turns
// the model's own probabilities for each digit into the reading, so the
// confidence is the model's, not a guess.
type Ollama struct {
	URL    string
	Model  string
	Client *http.Client
}

func NewOllama(url, model string) *Ollama {
	if url == "" {
		url = "http://127.0.0.1:11434"
	}
	if model == "" {
		model = DefaultOllamaModel
	}
	return &Ollama{URL: url, Model: model, Client: &http.Client{Timeout: 20 * time.Second}}
}

var descriptions = map[Mood]string{
	Frustrated:  "annoyed, stuck, something is failing or broken",
	Focused:     "working steadily, building or testing something",
	Celebrating: "something worked, finished, shipped or passed",
	Tired:       "worn out, late, wanting a break",
	Calm:        "neutral, nothing notable going on",
}

func prompt(e Event) string {
	var b strings.Builder
	b.WriteString("You read a developer's mood from one line of their terminal activity.\n")
	b.WriteString("Options:\n")
	for i, m := range All {
		fmt.Fprintf(&b, "%d. %s: %s\n", i+1, m, descriptions[m])
	}
	fmt.Fprintf(&b, "\nActivity (%s): %s\n\nAnswer with the option number only.", e.Kind, e.Text)
	return b.String()
}

type generateResponse struct {
	Logprobs []struct {
		TopLogprobs []struct {
			Token   string  `json:"token"`
			Logprob float64 `json:"logprob"`
		} `json:"top_logprobs"`
	} `json:"logprobs"`
}

func (o *Ollama) Read(e Event) (Reading, error) {
	body, err := json.Marshal(map[string]any{
		"model":        o.Model,
		"prompt":       prompt(e),
		"stream":       false,
		"think":        false,
		"logprobs":     true,
		"top_logprobs": 10,
		"options":      map[string]any{"num_predict": 1, "temperature": 0},
	})
	if err != nil {
		return Reading{}, err
	}
	resp, err := o.Client.Post(o.URL+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return Reading{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Reading{}, fmt.Errorf("ollama returned %s", resp.Status)
	}
	var out generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Reading{}, err
	}
	return parseDigits(out)
}

func parseDigits(out generateResponse) (Reading, error) {
	if len(out.Logprobs) == 0 {
		return Reading{}, fmt.Errorf("ollama returned no logprobs")
	}
	scores := make(map[Mood]float64, len(All))
	total := 0.0
	for _, t := range out.Logprobs[0].TopLogprobs {
		tok := strings.TrimSpace(t.Token)
		if len(tok) != 1 || tok[0] < '1' || int(tok[0]-'1') >= len(All) {
			continue
		}
		p := math.Exp(t.Logprob)
		scores[All[tok[0]-'1']] += p
		total += p
	}
	if total == 0 {
		return Reading{}, fmt.Errorf("model did not answer with an option number")
	}
	for m := range scores {
		scores[m] /= total
	}
	return Reading{Scores: scores}, nil
}

// Fallback uses Backup whenever Primary fails.
type Fallback struct {
	Primary, Backup Reader
}

func (f Fallback) Read(e Event) (Reading, error) {
	if r, err := f.Primary.Read(e); err == nil {
		return r, nil
	}
	return f.Backup.Read(e)
}
