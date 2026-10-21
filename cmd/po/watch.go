package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bkodzo/po/internal/config"
	"github.com/bkodzo/po/internal/ipc"
	"github.com/bkodzo/po/internal/theme"
)

// runWatch prints each state line the pet streams, until it stops or you quit.
func runWatch() {
	sub, err := ipc.Subscribe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "po: no pet to watch (is it running? try po start)")
		os.Exit(1)
	}
	defer sub.Close()
	for {
		line, err := sub.Next()
		if err != nil {
			return
		}
		fmt.Println(string(line))
	}
}

// runTheme handles `po theme export [path]`.
func runTheme(args []string) {
	if len(args) == 0 || args[0] != "export" || len(args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: po theme export [path]")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}
	th, ok := theme.Get(cfg.Theme)
	if !ok {
		fmt.Fprintf(os.Stderr, "po: unknown theme %q\n", cfg.Theme)
		os.Exit(1)
	}
	var path string
	if len(args) == 2 {
		path = args[1]
	} else {
		sock, err := ipc.Path()
		if err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		path = filepath.Join(filepath.Dir(sock), "theme.json")
	}
	if err := th.WriteFile(path); err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}
	fmt.Println("wrote", path)
}
