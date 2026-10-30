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
		fmt.Printf("the corner pet in terminals is %s\n", cfg.Overlay)
		if cfg.Overlay == "on" {
			fmt.Println("note: it is drawn with text characters, so copying text near it copies the pet too. `po overlay off` stops that; the desktop window (po window) has no such problem.")
		} else {
			fmt.Println("the desktop window is the main pet: start it with `po window`. `po overlay on` adds a small text pet in terminals.")
		}
		return
	}
	if len(args) != 1 || (args[0] != "on" && args[0] != "off") {
		fmt.Fprintln(os.Stderr, "usage: po overlay [on | off | status]")
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
		fmt.Println("the corner pet is off: nothing is drawn in your terminals (the desktop pet is po window)")
	} else {
		fmt.Println("the corner pet is on: it appears again at your next prompt")
	}
}
