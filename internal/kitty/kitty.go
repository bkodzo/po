// Package kitty builds the escape sequences of the Kitty graphics protocol,
// which lets a terminal draw real images that are not part of its text. That
// matters for Po's corner pet: it can sit on top of the terminal without being
// copied along with the text next to it.
//
// Only the core of the protocol is used (image data sent directly, placed,
// and deleted), so it works on every terminal that supports the protocol at
// all. Replies from the terminal are switched off, because they would arrive as
// typed input.
package kitty

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/bkodzo/po/internal/sprite"
)

// chunkSize is the most base64 text the protocol allows in one escape.
const chunkSize = 4096

// Supported reports whether a terminal, named as it names itself in
// TERM_PROGRAM, is known to draw Kitty graphics. It is deliberately a short
// list: a terminal that does not understand the sequences would print them as
// text, so unknown terminals are treated as unsupported.
func Supported(term string) bool {
	switch strings.ToLower(term) {
	case "ghostty", "kitty", "wezterm":
		return true
	}
	return false
}

// RGBA draws a canvas as raw 8-bit RGBA pixels, each sprite pixel becoming a
// block of blockW by blockH device pixels, so the picture stays sharp when the
// terminal shows it at exactly that size. Transparent pixels get zero alpha.
func RGBA(c *sprite.Canvas, pal sprite.Palette, blockW, blockH int) (data []byte, w, h int) {
	w, h = c.W*blockW, c.H*blockH
	data = make([]byte, w*h*4)
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			col, ok := pal[c.Get(x, y)]
			if !ok {
				continue
			}
			for by := 0; by < blockH; by++ {
				row := ((y*blockH+by)*w + x*blockW) * 4
				for bx := 0; bx < blockW; bx++ {
					i := row + bx*4
					data[i], data[i+1], data[i+2], data[i+3] = col.R, col.G, col.B, 255
				}
			}
		}
	}
	return data, w, h
}

// Transmit stores an RGBA image in the terminal under id without showing it.
func Transmit(id int, rgba []byte, w, h int) string {
	payload := base64.StdEncoding.EncodeToString(rgba)
	var b strings.Builder
	for start := 0; start < len(payload); start += chunkSize {
		end := start + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		more := 1
		if end == len(payload) {
			more = 0
		}
		if start == 0 {
			fmt.Fprintf(&b, "\x1b_Ga=t,f=32,s=%d,v=%d,i=%d,q=2,m=%d;%s\x1b\\", w, h, id, more, payload[start:end])
		} else {
			// After the first chunk only m (and q) may be given.
			fmt.Fprintf(&b, "\x1b_Gq=2,m=%d;%s\x1b\\", more, payload[start:end])
		}
	}
	return b.String()
}

// Place shows a stored image at the cursor, scaled to cols by rows cells. The
// cursor does not move. A positive z puts it above the text.
func Place(id, placement, cols, rows, z int) string {
	return fmt.Sprintf("\x1b_Ga=p,i=%d,p=%d,c=%d,r=%d,C=1,z=%d,q=2\x1b\\", id, placement, cols, rows, z)
}

// Remove takes a placement off the screen but keeps the image stored, so it can
// be placed again cheaply.
func Remove(id, placement int) string {
	return fmt.Sprintf("\x1b_Ga=d,d=i,i=%d,p=%d,q=2\x1b\\", id, placement)
}

// Free deletes a stored image and anything showing it, releasing its memory.
func Free(id int) string {
	return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id)
}
