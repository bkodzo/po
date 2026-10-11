package overlay

import "testing"

func TestValidTTYRejectsOtherPaths(t *testing.T) {
	for _, p := range []string{"", "/etc/passwd", "/dev/null", "/dev/tty/../../etc/passwd", "relative/tty", "/dev/ttyfake-does-not-exist"} {
		if ValidTTY(p) {
			t.Errorf("ValidTTY(%q) = true, want false", p)
		}
	}
}
