package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// runTheme handles `po theme list`, `po theme set <name>` and `po theme export`.
func runTheme(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}
	switch {
	case len(args) == 1 && args[0] == "list":
		for _, name := range theme.Names() {
			mark := " "
			if name == cfg.Theme {
				mark = "*"
			}
			fmt.Printf("%s %s\n", mark, name)
		}
	case len(args) == 2 && args[0] == "set":
		if _, ok := theme.Get(args[1]); !ok {
			fmt.Fprintf(os.Stderr, "po: no theme called %q (try: %s)\n", args[1], strings.Join(theme.Names(), ", "))
			os.Exit(1)
		}
		if err := cfg.Set("theme", args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		if err := cfg.Save(); err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		ipc.Send(ipc.Message{Op: "reload"})
		fmt.Printf("theme is now %s\n", args[1])
	case len(args) >= 1 && args[0] == "export" && len(args) <= 2:
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
	default:
		fmt.Fprintln(os.Stderr, "usage: po theme list | set <name> | export [path]")
		os.Exit(2)
	}
}
