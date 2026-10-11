package event

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/bkodzo/po/internal/mood"
)

// Command is one finished shell command, as reported by the shell hook.
type Command struct {
	Text   string `json:"text"`
	Exit   int    `json:"exit"`
	Millis int64  `json:"ms"`
}

const maxLen = 160

var trivial = map[string]bool{
	"ls": true, "cd": true, "pwd": true, "clear": true, "exit": true,
	"history": true, "fg": true, "bg": true, "jobs": true,
}

// Skip reports whether a command is too boring to bother the pet with.
// Commands typed with a leading space are private by shell convention.
func Skip(text string) bool {
	if strings.HasPrefix(text, " ") {
		return true
	}
	fields := strings.Fields(text)
	return len(fields) == 0 || trivial[fields[0]]
}

var redactions = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`), "://***@"},
	{regexp.MustCompile(`(?i)\b(\w*(?:token|secret|passw(?:or)?d|api[_-]?key|auth\w*))=\S+`), "$1=***"},
	{regexp.MustCompile(`(?i)(--?(?:token|password|passwd|secret|api-key|apikey|key)[ =])\S+`), "${1}***"},
	{regexp.MustCompile(`(?i)\bbearer\s+\S+`), "Bearer ***"},
	{regexp.MustCompile(`[A-Za-z0-9_\-+/=]{32,}`), "***"},
}

// Redact strips things that look like credentials.
func Redact(text string) string {
	for _, r := range redactions {
		text = r.re.ReplaceAllString(text, r.with)
	}
	return text
}

// ToEvent turns a finished command into something a mood reader can judge.
func (c Command) ToEvent() mood.Event {
	text := []rune(Redact(strings.TrimSpace(c.Text)))
	if len(text) > maxLen {
		text = append(text[:maxLen], '…')
	}
	var out string
	if c.Exit == 0 {
		out = fmt.Sprintf("ran ok: %s", string(text))
	} else {
		out = fmt.Sprintf("failed with exit code %d: %s", c.Exit, string(text))
	}
	if c.Millis >= 10_000 {
		out += fmt.Sprintf(" (took %ds)", c.Millis/1000)
	}
	return mood.Event{Kind: "command", Text: out}
}
