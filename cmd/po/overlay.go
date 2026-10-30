package main

import (
	"fmt"
	"os"

	"github.com/bkodzo/po/internal/config"
	"github.com/bkodzo/po/internal/ipc"
)

// runOverlay handles `po overlay on|off|status`: whether Po draws the small pet
// in the corner of your terminals.
func runOverlay(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}
	if len(args) == 0 || args[0] == "status" {
		fmt.Printf("the corner pet in terminals is %s, drawn as %s\n", cfg.Overlay, cfg.OverlayStyle)
		if cfg.Overlay == "on" && cfg.OverlayStyle == "text" {
			fmt.Println("note: text characters are copied along with nearby text. Try `po overlay style image` in a terminal that supports it (Ghostty, Kitty, WezTerm), or use the desktop window (po window).")
		}
		return
	}
	if args[0] == "style" {
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: po overlay style text|image|auto")
			os.Exit(2)
		}
		if err := cfg.Set("overlay_style", args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		if err := cfg.Save(); err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		ipc.Send(ipc.Message{Op: "reload"})
		switch args[1] {
		case "image":
			fmt.Println("the corner pet is now drawn as an image where the terminal supports it (Ghostty, Kitty, WezTerm), and as text elsewhere. Run a command to see it; if anything looks wrong, go back with: po overlay style text")
		case "auto":
			fmt.Println("the corner pet is drawn as an image where the terminal supports it, and as text elsewhere")
		default:
			fmt.Println("the corner pet is drawn as text characters again")
		}
		return
	}
	if len(args) != 1 || (args[0] != "on" && args[0] != "off") {
		fmt.Fprintln(os.Stderr, "usage: po overlay [on | off | status | style text|image|auto]")
		os.Exit(2)
	}
	if err := cfg.Set("overlay", args[0]); err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}
	if err := cfg.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}
	// Tell a running pet straight away; if none is running, the setting applies
	// when it starts.
	ipc.Send(ipc.Message{Op: "reload"})
	if args[0] == "off" {
		fmt.Println("the corner pet is off: nothing is drawn in your terminals any more (try po window for the desktop pet)")
	} else {
		fmt.Println("the corner pet is on: it appears again at your next prompt")
	}
}
