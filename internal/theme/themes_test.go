package theme

import (
	"strings"
	"testing"

	"github.com/bkodzo/po/internal/mood"
)

func eachTheme(t *testing.T, f func(t *testing.T, th Theme)) {
	t.Helper()
	for _, name := range Names() {
		th, ok := Get(name)
		if !ok {
			t.Fatalf("theme %q is listed but cannot be found", name)
		}
		t.Run(name, func(t *testing.T) { f(t, th) })
	}
}

func TestTheRegistryHasTheThemesAndTheyKnowTheirNames(t *testing.T) {
	names := Names()
	for _, want := range []string{"soccer", "ghost", "cat"} {
		found := false
		for _, n := range names {
			found = found || n == want
		}
		if !found {
			t.Errorf("theme %q is missing from %v", want, names)
		}
	}
	eachTheme(t, func(t *testing.T, th Theme) {
		if got, _ := Get(th.Name); got.Name != th.Name {
			t.Errorf("registered under %q but calls itself %q", th.Name, got.Name)
		}
	})
}

func TestEveryThemeHasWordsForEveryMood(t *testing.T) {
	eachTheme(t, func(t *testing.T, th Theme) {
		for _, m := range mood.All {
			if len(th.Lines[m]) == 0 {
				t.Errorf("no lines for %s", m)
			}
		}
		if len(th.Unsure) == 0 {
			t.Error("no lines for when it is unsure")
		}
	})
}

// A letter drawn but missing from the palette is simply invisible, which would
// leave a hole in the creature.
func TestEveryLetterDrawnHasAColour(t *testing.T) {
	eachTheme(t, func(t *testing.T, th Theme) {
		check := func(where string, letters string, pal map[rune]bool) {
			for _, r := range letters {
				if r != '.' && r != '\n' && !pal[r] {
					t.Fatalf("%s draws %q, which has no colour in the palette", where, r)
				}
			}
		}
		for _, m := range mood.All {
			for tick := 0; tick < 48; tick++ {
				c, pal := th.Frame(m, tick)
				have := map[rune]bool{}
				for k := range pal {
					have[k] = true
				}
				check(string(m)+" full", c.Letters(), have)
				mc, mp := th.MiniFrame(m, tick)
				have = map[rune]bool{}
				for k := range mp {
					have[k] = true
				}
				check(string(m)+" mini", mc.Letters(), have)
			}
		}
		for _, name := range ActivityNames() {
			info, _ := ActivityOf(name)
			for tick := 0; tick < info.Ticks; tick++ {
				c, pal := th.ActivityFrame(name, tick)
				have := map[rune]bool{}
				for k := range pal {
					have[k] = true
				}
				check(name, c.Letters(), have)
			}
		}
	})
}

func TestEveryThemeLooksDifferentInEachTintedMood(t *testing.T) {
	eachTheme(t, func(t *testing.T, th Theme) {
		_, calm := th.Frame(mood.Calm, 0)
		for _, m := range []mood.Mood{mood.Frustrated, mood.Celebrating, mood.Tired, mood.Focused} {
			_, pal := th.Frame(m, 0)
			if pal['b'] == calm['b'] {
				t.Errorf("%s has the same body colour as calm", m)
			}
		}
	})
}

func TestEveryThemeKeepsItsGlyphsInsideTheCanvas(t *testing.T) {
	eachTheme(t, func(t *testing.T, th Theme) {
		for tick := 0; tick < 64; tick++ {
			c, _ := th.Frame(mood.Tired, tick)
			if n := strings.Count(c.Letters(), "z"); n != 13 {
				t.Fatalf("tired tick %d: the Z has %d pixels, want 13", tick, n)
			}
			s, _ := th.ActivityFrame("sleep", tick)
			switch n := strings.Count(s.Letters(), "z"); n {
			case 0, 10, 13, 23:
			default:
				t.Fatalf("sleep tick %d: %d Z pixels, so a Z is clipped or overlapping", tick, n)
			}
		}
	})
}

func TestCatEarsAreNeverClipped(t *testing.T) {
	for _, m := range mood.All {
		for tick := 0; tick < 128; tick++ {
			c, _ := Cat.Frame(m, tick)
			if n := strings.Count(c.Letters(), "e"); n != 2 {
				t.Fatalf("%s tick %d: %d ear-inner pixels, want 2 (an ear is clipped at the top of the canvas)", m, tick, n)
			}
		}
	}
	for _, name := range ActivityNames() {
		info, _ := ActivityOf(name)
		for tick := 0; tick < info.Ticks; tick++ {
			c, _ := Cat.ActivityFrame(name, tick)
			if n := strings.Count(c.Letters(), "e"); n != 2 {
				t.Fatalf("%s tick %d: %d ear-inner pixels, want 2", name, tick, n)
			}
		}
	}
}

func TestGhostHasNoFeetAndFloatsAboveItsShadow(t *testing.T) {
	c, _ := Ghost.Frame(mood.Calm, 0)
	if strings.Contains(c.Letters(), "f") {
		t.Error("a ghost has no feet")
	}
	// The hem has gaps between its lobes, so the bottom row is not solid.
	rows := strings.Split(c.Letters(), "\n")
	hem := ""
	for y := len(rows) - 1; y >= 0; y-- {
		if strings.Contains(rows[y], "o") && !strings.Contains(rows[y], "h") {
			hem = rows[y]
			break
		}
	}
	if !strings.Contains(strings.Trim(hem, "."), ".") {
		t.Errorf("the hem should be scalloped with gaps, got %q", hem)
	}
}

func TestEveryThemeExportsAndTheExportMatchesTheDrawing(t *testing.T) {
	eachTheme(t, func(t *testing.T, th Theme) {
		pack := th.Export()
		if pack.Name != th.Name || len(pack.Available) < 3 {
			t.Fatalf("header wrong: %q, available %v", pack.Name, pack.Available)
		}
		for _, m := range mood.All {
			mp := pack.Moods[string(m)]
			for _, a := range []Animation{mp.Full, mp.Mini} {
				for _, f := range a.Frames {
					if len(f) != a.W*a.H {
						t.Fatalf("%s: frame has %d letters, want %d", m, len(f), a.W*a.H)
					}
				}
			}
			c, _ := th.Frame(m, 7)
			want := strings.ReplaceAll(c.Letters(), "\n", "")
			if got := string(mp.Full.Frames[mp.Full.Sequence[7%len(mp.Full.Sequence)]]); got != want {
				t.Errorf("%s: the pack's frame 7 differs from the drawing", m)
			}
		}
	})
}
