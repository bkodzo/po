package mood

import "testing"

func cmdEvent(cmd string, exit int, ms int64, streak, hour int) Event {
	return Event{Kind: "command", Cmd: cmd, Exit: exit, Millis: ms, FailStreak: streak, Hour: hour, Text: cmd}
}

func TestRulesTopMood(t *testing.T) {
	cases := []struct {
		name string
		e    Event
		want Mood
	}{
		{"a failure", cmdEvent("go test ./...", 1, 800, 0, 14), Frustrated},
		{"a long run of failures", cmdEvent("npm test", 1, 800, 5, 15), Frustrated},
		{"relief after failures", cmdEvent("go test ./...", 0, 800, 3, 14), Celebrating},
		{"a push", cmdEvent("git push", 0, 700, 0, 16), Celebrating},
		{"a deploy", cmdEvent("make deploy", 0, 20000, 0, 13), Celebrating},
		{"a plain look at status", cmdEvent("git status", 0, 40, 0, 14), Calm},
		{"a late night success", cmdEvent("git push", 0, 700, 0, 3), Tired},
		{"a late night failure is still annoyance", cmdEvent("go build", 1, 900, 0, 2), Frustrated},
		{"a Ctrl-C is not a failure", cmdEvent("go run .", 130, 5000, 0, 14), Calm},
		{"a typo is mild", cmdEvent("gti status", 127, 10, 0, 10), Frustrated},
		{"going to bed in a commit message", cmdEvent(`git commit -m "going to bed"`, 0, 200, 0, 23), Tired},
	}
	for _, tc := range cases {
		r, err := Rules{}.Read(tc.e)
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := r.Top(); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestRulesScoresSumToOne(t *testing.T) {
	r, _ := Rules{}.Read(cmdEvent("make", 0, 1000, 1, 12))
	sum := 0.0
	for _, m := range All {
		sum += r.Scores[m]
	}
	if sum < 0.999 || sum > 1.001 {
		t.Errorf("scores sum to %v", sum)
	}
}

func TestRulesFallBackToKeywordsForText(t *testing.T) {
	r, _ := Rules{}.Read(Event{Kind: "text", Text: "ugh the build failed again"})
	if got, _ := r.Top(); got != Frustrated {
		t.Errorf("got %s, want frustrated", got)
	}
}

func TestCodeInvestigationIsNotCalm(t *testing.T) {
	r, _ := Rules{}.Read(cmdEvent(`grep -rn "TODO" src/`, 0, 80, 0, 14))
	if got, _ := r.Top(); got != Focused {
		t.Errorf("got %s, want focused", got)
	}
}
