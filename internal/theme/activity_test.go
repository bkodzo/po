package theme

import (
	"strings"
	"testing"
)

func TestEveryActivityDrawsACompleteCreature(t *testing.T) {
	for _, name := range ActivityNames() {
		info, _ := ActivityOf(name)
		for tick := 0; tick < info.Ticks*2; tick++ {
			c, pal := Soccer.ActivityFrame(name, tick)
			if c.W != CanvasW || c.H != CanvasH {
				t.Fatalf("%s tick %d: canvas %dx%d", name, tick, c.W, c.H)
			}
			letters := c.Letters()
			for _, want := range []string{"b", "o", "f", "h"} { // body, outline, feet, shadow
				if !strings.Contains(letters, want) {
					t.Fatalf("%s tick %d: no %q pixels, so the creature is not fully drawn", name, tick, want)
				}
			}
			if strings.Count(c.Render(pal), "\n") != CanvasH/2-1 {
				t.Fatalf("%s tick %d: wrong row count", name, tick)
			}
		}
	}
}

func TestSleepLoopsAndOneShotsHoldTheirLastPicture(t *testing.T) {
	sleep, _ := ActivityOf("sleep")
	if !sleep.Loop {
		t.Fatal("sleeping should loop")
	}
	for tick := 0; tick < 2*sleep.Ticks; tick++ {
		a, _ := Soccer.ActivityFrame("sleep", tick)
		b, _ := Soccer.ActivityFrame("sleep", tick+sleep.Ticks)
		if a.Letters() != b.Letters() {
			t.Fatalf("sleep frame %d differs from frame %d, so the loop is wrong", tick, tick+sleep.Ticks)
		}
	}
	for _, name := range []string{"look", "stretch", "wake"} {
		info, _ := ActivityOf(name)
		if info.Loop {
			t.Errorf("%s should play once", name)
		}
		last, _ := Soccer.ActivityFrame(name, info.Ticks-1)
		after, _ := Soccer.ActivityFrame(name, info.Ticks+50)
		if last.Letters() != after.Letters() {
			t.Errorf("%s should hold its last picture after it ends", name)
		}
	}
}

func TestActivityGlyphsAreNeverClipped(t *testing.T) {
	for tick := 0; tick < 64; tick++ {
		c, _ := Soccer.ActivityFrame("sleep", tick)
		// The big Z has 13 pixels and the small one 10; a clipped one would
		// show a different count.
		switch n := strings.Count(c.Letters(), "z"); n {
		case 0, 10, 13, 23:
		default:
			t.Fatalf("sleep tick %d: %d Z pixels, so a Z is clipped or overlapping", tick, n)
		}
	}
	for tick := 0; tick < 10; tick++ {
		c, _ := Soccer.ActivityFrame("wake", tick)
		n := strings.Count(c.Letters(), "y")
		if tick >= 2 && tick < 8 && n != 4 {
			t.Errorf("wake tick %d: the exclamation mark has %d pixels, want 4", tick, n)
		}
		if (tick < 2 || tick >= 8) && n != 0 {
			t.Errorf("wake tick %d: the exclamation mark should be gone, found %d pixels", tick, n)
		}
	}
}

func TestLookActuallyLooksSideToSide(t *testing.T) {
	seen := map[string]bool{}
	for tick := 0; tick < 28; tick++ {
		c, _ := Soccer.ActivityFrame("look", tick)
		seen[c.Letters()] = true
	}
	if len(seen) < 3 {
		t.Errorf("look has only %d distinct pictures; it should glance left, right and back", len(seen))
	}
}

func TestExportIncludesTheActivities(t *testing.T) {
	pack := Soccer.Export()
	for _, name := range ActivityNames() {
		a, ok := pack.Activities[name]
		if !ok {
			t.Fatalf("activity %s missing from the pack", name)
		}
		info, _ := ActivityOf(name)
		if a.Loop != info.Loop || len(a.Sequence) != info.Ticks || len(a.Palette) == 0 {
			t.Errorf("%s exported wrongly: loop %v, %d ticks, %d colours", name, a.Loop, len(a.Sequence), len(a.Palette))
		}
		for _, i := range a.Sequence {
			if i < 0 || i >= len(a.Frames) {
				t.Fatalf("%s sequence points at frame %d of %d", name, i, len(a.Frames))
			}
		}
		// The pack's picture is the drawing's picture.
		c, _ := Soccer.ActivityFrame(name, 3)
		if a.Frames[a.Sequence[3]] != strings.ReplaceAll(c.Letters(), "\n", "") {
			t.Errorf("%s tick 3 differs between the pack and the drawing", name)
		}
	}
}
