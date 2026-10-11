package theme

import (
	"strings"
	"testing"

	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/sprite"
)

func TestEveryMoodDrawsEveryFrame(t *testing.T) {
	for _, m := range mood.All {
		for tick := 0; tick < 128; tick++ {
			c, pal := Soccer.Frame(m, tick)
			if c.W != CanvasW || c.H != CanvasH {
				t.Fatalf("%s tick %d: canvas %dx%d", m, tick, c.W, c.H)
			}
			if !strings.Contains(c.Letters(), "b") {
				t.Fatalf("%s tick %d: no body drawn", m, tick)
			}
			if strings.Count(c.Render(pal), "\n") != CanvasH/2-1 {
				t.Fatalf("%s tick %d: wrong row count", m, tick)
			}
		}
	}
}

func TestTintChangesBodyColour(t *testing.T) {
	_, calm := Soccer.Frame(mood.Calm, 0)
	_, angry := Soccer.Frame(mood.Frustrated, 0)
	if calm['b'] == angry['b'] {
		t.Error("frustrated body should differ from calm")
	}
}

func TestUnknownPaletteKeyIsClear(t *testing.T) {
	c := sprite.NewCanvas(2, 2)
	c.Set(0, 0, '?')
	if strings.Contains(c.Render(sprite.Palette{}), "▀") {
		t.Error("unknown keys should render as empty space")
	}
}

func TestMiniFrameFitsTwoRows(t *testing.T) {
	for _, m := range mood.All {
		for tick := 0; tick < 80; tick++ {
			c, pal := Soccer.MiniFrame(m, tick)
			if c.W != MiniW || c.H != 4 {
				t.Fatalf("%s tick %d: canvas %dx%d, want %dx4", m, tick, c.W, c.H, MiniW)
			}
			if !strings.Contains(c.Letters(), "b") {
				t.Fatalf("%s tick %d: no body", m, tick)
			}
			if rows := strings.Count(c.Render(pal), "\n") + 1; rows != 2 {
				t.Fatalf("%s tick %d: %d terminal rows, want 2", m, tick, rows)
			}
		}
	}
}
