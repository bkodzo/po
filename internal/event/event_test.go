package event

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	cases := map[string]string{
		"export API_TOKEN=abc123def":                       "export API_TOKEN=***",
		"curl -H 'Authorization: Bearer sk-live-9f8e7d6c'": "Bearer ***",
		"deploy --password hunter2 prod":                   "deploy --password *** prod",
		"git clone https://bob:s3cret@github.com/x/y.git":  "https://***@github.com",
		"echo AbCdEfGhIjKlMnOpQrStUvWxYz0123456789":        "echo ***",
	}
	for in, want := range cases {
		got := Redact(in)
		if !strings.Contains(got, want) {
			t.Errorf("Redact(%q) = %q, want it to contain %q", in, got, want)
		}
		for _, secret := range []string{"abc123def", "sk-live", "hunter2", "s3cret", "AbCdEf"} {
			if strings.Contains(got, secret) {
				t.Errorf("Redact(%q) leaked %q: %q", in, secret, got)
			}
		}
	}
}

func TestRedactLeavesOrdinaryCommands(t *testing.T) {
	for _, cmd := range []string{"go test ./...", "git commit -m \"fix parser\"", "npm run build"} {
		if got := Redact(cmd); got != cmd {
			t.Errorf("Redact(%q) changed it to %q", cmd, got)
		}
	}
}

func TestSkip(t *testing.T) {
	for cmd, want := range map[string]bool{
		"ls -la": true, "cd ..": true, "": true, "  ": true,
		" secret-thing": true, "go test ./...": false, "git push": false,
	} {
		if got := Skip(cmd); got != want {
			t.Errorf("Skip(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestToEvent(t *testing.T) {
	fail := Command{Text: "go test ./...", Exit: 1}.ToEvent()
	if !strings.Contains(fail.Text, "failed with exit code 1") {
		t.Errorf("failure not described: %q", fail.Text)
	}
	ok := Command{Text: "go build", Exit: 0, Millis: 42_000}.ToEvent()
	if !strings.Contains(ok.Text, "ran ok") || !strings.Contains(ok.Text, "took 42s") {
		t.Errorf("success not described: %q", ok.Text)
	}
	long := Command{Text: strings.Repeat("a b ", 100)}.ToEvent()
	if len([]rune(long.Text)) > maxLen+40 {
		t.Errorf("long command not truncated: %d runes", len([]rune(long.Text)))
	}
}
