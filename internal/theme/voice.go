package theme

import (
	"fmt"
	"strings"
)

// MilestoneKinds are the notable moments the pet can remark on. They match the
// kinds the memory reports.
var MilestoneKinds = []string{"record", "relief", "late_run", "comeback", "busy", "first_push"}

// RecapTones are the kinds of week the recap can describe.
var RecapTones = []string{"quiet", "rough", "steady", "great"}

// Fallbacks are used for a theme that has no lines of its own for something.
var fallbackMilestones = map[string][]string{
	"record":     {"%d failures in a row. That is a record."},
	"relief":     {"First success after %d failures. Well done."},
	"late_run":   {"%d late nights in a row."},
	"comeback":   {"Welcome back. %d hours away."},
	"busy":       {"%d commands today."},
	"first_push": {"First push of the day."},
}

var fallbackRecaps = map[string][]string{
	"quiet":  {"A quiet week."},
	"rough":  {"A rough week, but you kept going."},
	"steady": {"A steady week."},
	"great":  {"A great week."},
}

// MilestoneLine is what the pet says for a moment of the given kind. n fills the
// number in the line, if it has one.
func (t Theme) MilestoneLine(kind string, n int) string {
	lines := t.Milestones[kind]
	if len(lines) == 0 {
		lines = fallbackMilestones[kind]
	}
	if len(lines) == 0 {
		return ""
	}
	line := pick(lines)
	if strings.Contains(line, "%d") {
		return fmt.Sprintf(line, n)
	}
	return line
}

// RecapLine is the pet's verdict on a week of the given tone.
func (t Theme) RecapLine(tone string) string {
	lines := t.Recaps[tone]
	if len(lines) == 0 {
		lines = fallbackRecaps[tone]
	}
	if len(lines) == 0 {
		return ""
	}
	return pick(lines)
}
