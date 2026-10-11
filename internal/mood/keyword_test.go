package mood

import "testing"

func TestKeywordTopMood(t *testing.T) {
	cases := map[string]Mood{
		"ugh the build failed again": Frustrated,
		"tests passed, finally":      Celebrating,
		"so tired, need sleep":       Tired,
	}
	for text, want := range cases {
		r, _ := Keyword{}.Read(Event{Text: text})
		got, _ := r.Top()
		if got != want {
			t.Errorf("%q: got %s, want %s", text, got, want)
		}
	}
}

func TestKeywordNoMatchIsUnsure(t *testing.T) {
	r, _ := Keyword{}.Read(Event{Text: "hello"})
	_, conf := r.Top()
	if conf >= 0.45 {
		t.Errorf("expected low confidence, got %v", conf)
	}
}
