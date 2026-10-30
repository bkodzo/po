package kitty

import (
	"encoding/base64"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/bkodzo/po/internal/sprite"
)

func TestSupportedIsAShortList(t *testing.T) {
	for term, want := range map[string]bool{
		"ghostty": true, "Ghostty": true, "kitty": true, "WezTerm": true,
		"Apple_Terminal": false, "iTerm.app": false, "tmux": false, "vscode": false,
		"Alacritty": false, "": false,
	} {
		if got := Supported(term); got != want {
			t.Errorf("Supported(%q) = %v, want %v", term, got, want)
		}
	}
}

func canvas() (*sprite.Canvas, sprite.Palette) {
	c := sprite.NewCanvas(2, 2)
	c.Set(0, 0, 'a')
	c.Set(1, 1, 'b')
	return c, sprite.Palette{'a': sprite.RGB(10, 20, 30), 'b': sprite.RGB(200, 100, 50)}
}

func TestRGBAScalesBlocksAndKeepsTransparency(t *testing.T) {
	c, pal := canvas()
	data, w, h := RGBA(c, pal, 3, 2)
	if w != 6 || h != 4 || len(data) != 6*4*4 {
		t.Fatalf("size %dx%d with %d bytes", w, h, len(data))
	}
	px := func(x, y int) [4]byte {
		i := (y*w + x) * 4
		return [4]byte{data[i], data[i+1], data[i+2], data[i+3]}
	}
	// The first sprite pixel fills a 3x2 block at the top left.
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			if px(x, y) != [4]byte{10, 20, 30, 255} {
				t.Errorf("pixel (%d,%d) = %v, want the first colour", x, y, px(x, y))
			}
		}
	}
	// The second is the block at the bottom right.
	if px(5, 3) != [4]byte{200, 100, 50, 255} || px(3, 2) != [4]byte{200, 100, 50, 255} {
		t.Error("bottom-right block has the wrong colour")
	}
	// Everything else is transparent.
	if px(3, 0)[3] != 0 || px(0, 2)[3] != 0 {
		t.Error("empty sprite pixels must be fully transparent")
	}
}

var apc = regexp.MustCompile(`(?s)\x1b_G([^;\x1b]*)(?:;([^\x1b]*))?\x1b\\`)

func TestTransmitRoundTripsAndFollowsTheChunkingRules(t *testing.T) {
	// Large enough to need several chunks.
	rgba := make([]byte, 70*50*4)
	for i := range rgba {
		rgba[i] = byte(i * 7)
	}
	out := Transmit(9101, rgba, 70, 50)
	parts := apc.FindAllStringSubmatch(out, -1)
	if len(parts) < 3 {
		t.Fatalf("expected several chunks, got %d", len(parts))
	}
	var joined strings.Builder
	for i, p := range parts {
		keys, payload := p[1], p[2]
		if len(payload) > 4096 {
			t.Errorf("chunk %d has %d characters, more than the 4096 allowed", i, len(payload))
		}
		if !strings.Contains(keys, "q=2") {
			t.Errorf("chunk %d does not silence replies: %q", i, keys)
		}
		last := i == len(parts)-1
		if last != strings.Contains(keys, "m=0") || (!last && !strings.Contains(keys, "m=1")) {
			t.Errorf("chunk %d has the wrong more-flag: %q", i, keys)
		}
		if i == 0 {
			for _, want := range []string{"a=t", "f=32", "s=70", "v=50", "i=9101"} {
				if !strings.Contains(keys, want) {
					t.Errorf("first chunk is missing %s: %q", want, keys)
				}
			}
		} else if strings.Contains(keys, "a=") || strings.Contains(keys, "i=") {
			t.Errorf("later chunks may only carry m and q, got %q", keys)
		}
		joined.WriteString(payload)
	}
	back, err := base64.StdEncoding.DecodeString(joined.String())
	if err != nil || len(back) != len(rgba) {
		t.Fatalf("payload did not decode back: %v (%d bytes)", err, len(back))
	}
	for i := range rgba {
		if back[i] != rgba[i] {
			t.Fatalf("byte %d differs after the round trip", i)
		}
	}
}

func TestPlaceRemoveFree(t *testing.T) {
	if got := Place(5, 1, 8, 2, 1); got != "\x1b_Ga=p,i=5,p=1,c=8,r=2,C=1,z=1,q=2\x1b\\" {
		t.Errorf("Place = %q", got)
	}
	for name, seq := range map[string]string{"Place": Place(5, 1, 8, 2, 1), "Remove": Remove(5, 1), "Free": Free(5)} {
		if !strings.Contains(seq, "q=2") {
			t.Errorf("%s does not silence the terminal's reply, which would arrive as typed input", name)
		}
	}
	if !strings.Contains(Place(5, 1, 8, 2, 1), "C=1") {
		t.Error("placing must not move the cursor")
	}
	if !strings.Contains(Remove(5, 1), "d=i") {
		t.Error("Remove must keep the stored image (lowercase d=i)")
	}
	if !strings.Contains(Free(5), "d=I") {
		t.Error("Free must delete the stored image (uppercase d=I)")
	}
	_ = strconv.Itoa
}
