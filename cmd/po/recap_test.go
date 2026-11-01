package main

import (
	"strings"
	"testing"

	"github.com/bkodzo/po/internal/memory"
	"github.com/bkodzo/po/internal/theme"
)

func TestRecapToneMatchesTheWeek(t *testing.T) {
	for name, tc := range map[string]struct {
		w    memory.Week
		want string
	}{
		"hardly anything":        {memory.Week{Commands: 10}, "quiet"},
		"many failures":          {memory.Week{Commands: 200, Failures: 50}, "rough"},
		"a very long streak":     {memory.Week{Commands: 500, Failures: 20, LongestStreak: 9}, "rough"},
		"clean with pushes":      {memory.Week{Commands: 300, Failures: 6, Pushes: 5}, "great"},
		"clean but no pushes":    {memory.Week{Commands: 300, Failures: 6, Pushes: 1}, "steady"},
		"in between":             {memory.Week{Commands: 300, Failures: 30, Pushes: 4}, "steady"},
		"the line between rough": {memory.Week{Commands: 100, Failures: 15}, "steady"},
		"just over the line":     {memory.Week{Commands: 100, Failures: 16}, "rough"},
	} {
		if got := recapTone(tc.w); got != tc.want {
			t.Errorf("%s: tone %q, want %q", name, got, tc.want)
		}
	}
}

func TestThousandsSeparators(t *testing.T) {
	for n, want := range map[int]string{0: "0", 7: "7", 999: "999", 1000: "1,000", 1482: "1,482", 1234567: "1,234,567"} {
		if got := thousands(n); got != want {
			t.Errorf("thousands(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestRecapShowsTheWeekInPlainWords(t *testing.T) {
	w := memory.Week{Days: 7, ActiveDays: 5, Commands: 1482, Failures: 94, LongestStreak: 9, LongestOn: "Tuesday",
		LateNights: 2, Reliefs: 4, Pushes: 11, BusiestDay: "Wednesday", BusiestCount: 904}
	out := formatRecap(w, theme.Soccer)
	for _, want := range []string{
		"the last 7 days", "active on 5 of 7 days", "1,482 commands, 94 failures (6%)",
		"longest run of failures: 9, on Tuesday", "late-night work", "first greens after a rough patch: 4",
		"pushes and deploys: 11", "busiest day: Wednesday, 904 commands",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("recap is missing %q:\n%s", want, out)
		}
	}
	// It ends with one of the character's lines for that kind of week.
	found := false
	for _, line := range theme.Soccer.Recaps[recapTone(w)] {
		found = found || strings.Contains(out, line)
	}
	if !found {
		t.Errorf("no line in the pet's voice:\n%s", out)
	}
}

func TestRecapSaysNothingYetWhenThereIsNothing(t *testing.T) {
	out := formatRecap(memory.Week{Days: 7}, theme.Soccer)
	if !strings.Contains(out, "Nothing remembered") || strings.Contains(out, "active on") {
		t.Errorf("unexpected:\n%s", out)
	}
}

func TestRecapSingularsAndOmissions(t *testing.T) {
	w := memory.Week{Days: 1, ActiveDays: 1, Commands: 1, Failures: 1}
	out := formatRecap(w, theme.Soccer)
	if !strings.Contains(out, "today") || !strings.Contains(out, "1 command, 1 failure") {
		t.Errorf("singulars wrong:\n%s", out)
	}
	for _, absent := range []string{"busiest day", "pushes", "late-night", "first greens", "longest run"} {
		if strings.Contains(out, absent) {
			t.Errorf("%q should be left out when there is nothing to say:\n%s", absent, out)
		}
	}
}
