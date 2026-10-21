package theme

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bkodzo/po/internal/mood"
)

func TestLoopsReallyRepeat(t *testing.T) {
	for _, m := range mood.All {
		for _, tc := range []struct {
			name   string
			period int
			frame  func(int) string
		}{
			{"full", fullPeriod[m], func(tick int) string { c, _ := Soccer.Frame(m, tick); return c.Letters() }},
			{"mini", miniPeriod[m], func(tick int) string { c, _ := Soccer.MiniFrame(m, tick); return c.Letters() }},
		} {
			for tick := 0; tick < 3*tc.period; tick++ {
				if tc.frame(tick) != tc.frame(tick+tc.period) {
					t.Fatalf("%s %s: frame %d differs from frame %d, so the loop of %d ticks is wrong",
						m, tc.name, tick, tick+tc.period, tc.period)
				}
			}
		}
	}
}

func TestExportMatchesTheDrawing(t *testing.T) {
	pack := Soccer.Export()
	if pack.Version != PackVersion || pack.Name != "soccer" {
		t.Fatalf("header wrong: %+v", pack)
	}
	for _, m := range mood.All {
		mp, ok := pack.Moods[string(m)]
		if !ok {
			t.Fatalf("mood %s missing", m)
		}
		for name, a := range map[string]Animation{"full": mp.Full, "mini": mp.Mini} {
			if len(a.Sequence) == 0 || len(a.Frames) == 0 {
				t.Fatalf("%s %s is empty", m, name)
			}
			for _, f := range a.Frames {
				if len(f) != a.W*a.H {
					t.Fatalf("%s %s: frame has %d letters, want %d", m, name, len(f), a.W*a.H)
				}
			}
			for _, i := range a.Sequence {
				if i < 0 || i >= len(a.Frames) {
					t.Fatalf("%s %s: sequence points at frame %d of %d", m, name, i, len(a.Frames))
				}
			}
		}
		// A frame played from the pack is exactly the frame the drawing gives.
		c, _ := Soccer.Frame(m, 5)
		a := mp.Full
		want := ""
		for _, row := range splitRows(c.Letters()) {
			want += row
		}
		if got := a.Frames[a.Sequence[5%len(a.Sequence)]]; got != want {
			t.Errorf("%s: frame 5 of the pack differs from the drawing", m)
		}
		if len(mp.Palette) == 0 {
			t.Errorf("%s has no palette", m)
		}
	}
	if len(pack.Lines["frustrated"]) == 0 {
		t.Error("lines missing")
	}
}

func splitRows(s string) []string {
	var rows []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			rows = append(rows, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(rows, cur)
}

func TestWriteFileProducesReadableJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "theme.json")
	if err := Soccer.WriteFile(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var back Pack
	if err := json.Unmarshal(data, &back); err != nil || back.Name != "soccer" {
		t.Fatalf("file did not read back: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("temporary file left behind")
	}
	t.Logf("pack size: %d KB", len(data)/1024)
}
