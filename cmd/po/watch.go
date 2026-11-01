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
		installed := map[string]bool{}
		for _, n := range theme.Installed() {
			installed[n] = true
		}
		for _, name := range theme.Names() {
			mark, kind := " ", ""
			if name == cfg.Theme {
				mark = "*"
			}
			if installed[name] {
				kind = "  (installed)"
			}
			fmt.Printf("%s %s%s\n", mark, name, kind)
		}
	case len(args) == 2 && args[0] == "validate":
		data, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		p, issues, err := theme.ParseAndValidate(data)
		if err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		if len(issues) > 0 {
			fmt.Printf("%s has %d problem(s):\n", args[1], len(issues))
			for _, i := range issues {
				fmt.Println("  " + i)
			}
			os.Exit(1)
		}
		kind := "a full pack"
		if p.Base != "" {
			kind = "a reskin of " + p.Base
		}
		fmt.Printf("%s is valid: %q, %s\n", args[1], p.Name, kind)
	case len(args) == 2 && args[0] == "install":
		name, err := theme.Install(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		fmt.Printf("installed %q; use it with: po theme set %s\n", name, name)
	case len(args) == 2 && args[0] == "remove":
		if err := theme.Remove(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		if cfg.Theme == args[1] { // don't leave the pet using a theme that is gone
			cfg.Theme = "soccer"
			cfg.Save()
			ipc.Send(ipc.Message{Op: "reload"})
			fmt.Printf("removed %q; it was in use, so the pet is back to soccer\n", args[1])
			return
		}
		fmt.Printf("removed %q\n", args[1])
	case len(args) >= 2 && args[0] == "new":
		base := "soccer"
		if len(args) == 4 && args[2] == "--base" {
			base = args[3]
		} else if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: po theme new <name> [--base soccer|ghost|cat]")
			os.Exit(2)
		}
		data, err := theme.Scaffold(args[1], base)
		if err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		path := args[1] + ".json"
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(os.Stderr, "po: %s already exists, so I left it alone\n", path)
			os.Exit(1)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s, a copy of %q to change. Edit the colours and lines, then:\n  po theme validate %s\n  po theme install %s\n  po theme set %s\n", path, base, path, path, args[1])
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
		fmt.Fprintln(os.Stderr, "usage: po theme list | set <name> | new <name> | validate <file> | install <file> | remove <name> | export [path]")
		os.Exit(2)
	}
}
