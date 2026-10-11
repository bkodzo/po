package mood

import (
	"encoding/json"
	"testing"
)

func TestParseDigitsNormalises(t *testing.T) {
	var out generateResponse
	raw := `{"logprobs":[{"top_logprobs":[
		{"token":"1","logprob":-0.1},
		{"token":" 3","logprob":-2.3},
		{"token":"The","logprob":-3.0}]}]}`
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	r, err := parseDigits(out)
	if err != nil {
		t.Fatal(err)
	}
	top, conf := r.Top()
	if top != Frustrated {
		t.Errorf("top = %s, want frustrated", top)
	}
	if conf < 0.85 || conf > 0.95 {
		t.Errorf("conf = %v, want about 0.9", conf)
	}
	if r.Scores[Celebrating] == 0 {
		t.Error("option 3 should map to celebrating")
	}
}

func TestParseDigitsRejectsNonAnswer(t *testing.T) {
	var out generateResponse
	json.Unmarshal([]byte(`{"logprobs":[{"top_logprobs":[{"token":"Sure","logprob":-0.1}]}]}`), &out)
	if _, err := parseDigits(out); err == nil {
		t.Error("expected an error when no option digit is present")
	}
}
