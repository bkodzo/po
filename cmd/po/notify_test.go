package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestAPrivateCommandIsNeverSent(t *testing.T) {
	for _, args := range [][]string{
		{" export SECRET=hunter2"},
		{"  rm -rf build"},
		{" ", "ls"}, // a leading space made by joining the words
		{""},
		{"   "},
		nil,
	} {
		if c := commandFrom(args, 1, 0); c != nil {
			t.Errorf("commandFrom(%q) = %+v, want nothing: a private or empty command must not leave the shell hook", args, c)
		}
	}
}

func TestOrdinaryCommandsAreSentIncludingTheDullOnes(t *testing.T) {
	for _, tc := range []struct {
		args []string
		text string
	}{
		{[]string{"go test ./..."}, "go test ./..."},
		{[]string{"ls"}, "ls"},                                  // dull, but it still means someone is here
		{[]string{"cd", "src"}, "cd src"},                       // the shell may pass the words separately
		{[]string{"git commit -m 'x'   "}, "git commit -m 'x'"}, // trailing space is trimmed
	} {
		c := commandFrom(tc.args, 0, 5)
		if c == nil || c.Text != tc.text || c.Millis != 5 {
			t.Errorf("commandFrom(%q) = %+v, want text %q", tc.args, c, tc.text)
		}
	}
	if c := commandFrom([]string{"false"}, 1, 0); c == nil || c.Exit != 1 {
		t.Errorf("the exit status must be carried: %+v", c)
	}
}

// The hook scripts must at least parse in the real shells, so a typo is caught
// before anyone loads one. A shell that is not installed is skipped.
func TestHookScriptsParseInTheirShells(t *testing.T) {
	for shell, args := range map[string][]string{
		"zsh":  {"zsh", "-n"},
		"bash": {"bash", "-n"},
		"fish": {"fish", "--no-execute"},
	} {
		t.Run(shell, func(t *testing.T) {
			bin, err := exec.LookPath(args[0])
			if err != nil {
				t.Skipf("%s is not installed", shell)
			}
			script, ok := hookScript(shell)
			if !ok || script == "" {
				t.Fatalf("no hook for %s", shell)
			}
			cmd := exec.Command(bin, args[1:]...)
			cmd.Stdin = strings.NewReader(script)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("the %s hook does not parse: %v\n%s", shell, err, out)
			}
		})
	}
	if _, ok := hookScript("tcsh"); ok {
		t.Error("an unsupported shell should have no hook")
	}
}
