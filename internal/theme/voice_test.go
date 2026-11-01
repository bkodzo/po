package theme

import (
	"strings"
	"testing"
)

func TestEveryThemeHasLinesForEveryMilestoneAndEveryKindOfWeek(t *testing.T) {
	eachTheme(t, func(t *testing.T, th Theme) {
		for _, kind := range MilestoneKinds {
			if len(th.Milestones[kind]) < 2 {
				t.Errorf("milestone %q has %d lines, want at least 2 so it is not always the same", kind, len(th.Milestones[kind]))
			}
		}
		for _, tone := range RecapTones {
			if len(th.Recaps[tone]) < 2 {
				t.Errorf("recap %q has %d lines, want at least 2", tone, len(th.Recaps[tone]))
			}
		}
	})
}

func TestMilestoneLinesFillInTheirNumberCleanly(t *testing.T) {
	eachTheme(t, func(t *testing.T, th Theme) {
		for kind, lines := range th.Milestones {
			for _, line := range lines {
				if n := strings.Count(line, "%d"); n > 1 {
					t.Errorf("%s: %q has %d placeholders, at most one is allowed", kind, line, n)
				}
				got := th.MilestoneLine(kind, 7) // a different line may be chosen; check all by formatting here
				if strings.Contains(got, "%!") || strings.Contains(got, "%d") {
					t.Errorf("%s: %q did not format cleanly", kind, got)
				}
			}
		}
		for _, kind := range []string{"record", "relief", "late_run", "comeback", "busy"} {
			numbered := false
			for _, line := range th.Milestones[kind] {
				numbered = numbered || strings.Contains(line, "%d")
			}
			if !numbered {
				t.Errorf("%s never mentions its number, so it says less than it could", kind)
			}
		}
	})
}

func TestAThemeWithNoLinesStillSaysSomething(t *testing.T) {
	bare := Theme{Name: "bare"}
	for _, kind := range MilestoneKinds {
		if bare.MilestoneLine(kind, 4) == "" {
			t.Errorf("no fallback for milestone %q", kind)
		}
	}
	for _, tone := range RecapTones {
		if bare.RecapLine(tone) == "" {
			t.Errorf("no fallback for recap %q", tone)
		}
	}
	if got := bare.MilestoneLine("relief", 5); !strings.Contains(got, "5") {
		t.Errorf("the number was not filled in: %q", got)
	}
	if bare.MilestoneLine("no-such-kind", 1) != "" {
		t.Error("an unknown kind should say nothing")
	}
}
