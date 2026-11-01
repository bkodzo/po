package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/bkodzo/po/internal/ipc"
	"github.com/bkodzo/po/internal/theme"
)

// runIdle handles `po idle <name>`: play one of the pet's idle animations now.
// Mostly for trying them out, since Po chooses them by itself.
func runIdle(args []string) {
	names := theme.ActivityNames()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: po idle <%s>\n", strings.Join(names, "|"))
		os.Exit(2)
	}
	if _, ok := theme.ActivityOf(args[0]); !ok {
		fmt.Fprintf(os.Stderr, "po: unknown animation %q (try: %s)\n", args[0], strings.Join(names, ", "))
		os.Exit(2)
	}
	if err := ipc.Send(ipc.Message{Op: "idle", Name: args[0]}); err != nil {
		fmt.Fprintln(os.Stderr, "po: Po is not running (try po start)")
		os.Exit(1)
	}
	fmt.Println("playing", args[0])
}
