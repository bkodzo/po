package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bkodzo/po/internal/config"
	"github.com/bkodzo/po/internal/daemon"
	"github.com/bkodzo/po/internal/ipc"
	"github.com/bkodzo/po/internal/stack"
)

func runStatus() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}

	raw, err := ipc.Ask(ipc.Message{Op: "status"})
	var st *daemon.Status
	if err == nil && raw != nil {
		st = new(daemon.Status)
		if json.Unmarshal(raw, st) != nil {
			st = nil
		}
	}

	switch {
	case st != nil:
		fmt.Printf("po is running (up %s), mood: %s\n", duration(st.UptimeSeconds), st.Mood)
	case stopped():
		fmt.Println("po is stopped on purpose (run `po start`)")
	default:
		fmt.Println("po is not running (it starts at your next prompt)")
	}

	fmt.Printf("reader: %s\n", describeReader(cfg))
	if cfg.Reader == "keyword" || cfg.Reader == "rules" || cfg.URL == "" {
		fmt.Println("model:  none; nothing leaves this machine")
	} else {
		local := stack.IsLocal(cfg.URL)
		state := "not reachable"
		switch {
		case st != nil && st.Model != nil && st.Model.Reachable:
			state = "reachable"
		case st == nil && stack.Reachable(cfg.URL):
			state = "reachable"
		}
		where := "hosted: redacted commands are sent to " + hostOf(cfg.URL)
		if local {
			where = "local: nothing leaves this machine"
		}
		fmt.Printf("model:  %s at %s (%s)\n        %s\n", cfg.Model, cfg.URL, state, where)
		if !local && cfg.ReadKey() == "" {
			fmt.Printf("        warning: no API key found (looked in $%s", cfg.KeyEnv)
			if cfg.KeyFile != "" {
				fmt.Printf(" and %s", cfg.KeyFile)
			}
			fmt.Println(")")
		}
	}

	if st != nil {
		fmt.Printf("events: %d handled; fail streak %d; shown in %d terminal(s)\n", st.Events, st.FailStreak, st.Terminals)
		if cfg.Reader == "auto" {
			fmt.Printf("answers: %d by the fast layer, %d by the model, %d where the model was unavailable\n",
				st.AnsweredFast, st.AnsweredModel, st.ModelFailed)
		}
	}
	fmt.Printf("config: %s\n", config.Path())
}

func describeReader(cfg config.Config) string {
	switch cfg.Reader {
	case "auto":
		return fmt.Sprintf("auto: rules answer when sure (gate %.2f), the model decides the rest", cfg.Gate)
	case "rules":
		return "rules only"
	case "keyword":
		return "keywords only"
	case "decision":
		return "decision model, rules if it fails"
	case "ollama":
		return "ollama model, rules if it fails"
	}
	return cfg.Reader
}

func hostOf(rawURL string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://")
	host, _, _ := strings.Cut(s, "/")
	return host
}

func duration(seconds int) string {
	switch {
	case seconds < 90:
		return fmt.Sprintf("%ds", seconds)
	case seconds < 5400:
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%dh", seconds/3600)
}

func runConfig(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}
	switch {
	case len(args) == 0:
		fmt.Printf("# %s\n", config.Path())
		for _, k := range config.Keys() {
			v, _ := cfg.Get(k)
			fmt.Printf("%s = %s\n", k, v)
		}
	case args[0] == "get" && len(args) == 2:
		v, err := cfg.Get(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		fmt.Println(v)
	case args[0] == "set" && len(args) == 3:
		if err := cfg.Set(args[1], args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		if err := cfg.Save(); err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		fmt.Printf("%s = %s\n", args[1], args[2])
		if args[1] == "url" && !stack.IsLocal(args[2]) && args[2] != "" {
			fmt.Printf("note: Po will send redacted commands to %s. Restart Po for this to take effect (po stop, then po start).\n", hostOf(args[2]))
		} else {
			fmt.Println("restart Po for this to take effect (po stop, then po start)")
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: po config [get <key> | set <key> <value>]")
		os.Exit(2)
	}
}
