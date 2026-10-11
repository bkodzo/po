package mood

import (
	"encoding/json"
	"testing"
)

func TestParseDecision(t *testing.T) {
	raw := `{"answers":{"mood":{"type":"choice","choice":"tired","confidence":0.8,
		"probabilities":{"calm":0.1,"celebrating":0.05,"focused":0.05,"frustrated":0.1,"tired":0.7}}}}`
	var out decisionResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	r, err := parseDecision(out)
	if err != nil {
		t.Fatal(err)
	}
	if top, conf := r.Top(); top != Tired || conf < 0.69 || conf > 0.71 {
		t.Errorf("top = %s %.2f, want tired 0.70", top, conf)
	}
}

func TestParseDecisionMissingAnswer(t *testing.T) {
	if _, err := parseDecision(decisionResponse{}); err == nil {
		t.Error("expected an error for an empty response")
	}
}
