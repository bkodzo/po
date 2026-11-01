package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bkodzo/po/internal/config"
	"github.com/bkodzo/po/internal/ipc"
	"github.com/bkodzo/po/internal/memory"
	"github.com/bkodzo/po/internal/theme"
)

// recapTone says what kind of week it was.
func recapTone(w memory.Week) string {
	if w.Commands < 30 {
		return "quiet"
	}
	rate := float64(w.Failures) / float64(w.Commands)
	switch {
	case rate > 0.15 || w.LongestStreak >= 8:
		return "rough"
	case rate < 0.05 && w.Pushes >= 3:
		return "great"
	}
	return "steady"
}

// thousands writes 1482 as 1,482.
func thousands(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// formatRecap writes the summary of a week, ending with a line in the pet's voice.
func formatRecap(w memory.Week, th theme.Theme) string {
	var b strings.Builder
	if w.Days == 1 {
		fmt.Fprintf(&b, "Po recap: today\n\n")
	} else {
		fmt.Fprintf(&b, "Po recap: the last %d days\n\n", w.Days)
	}
	if w.Commands == 0 {
		b.WriteString("  Nothing remembered for these days yet. Po remembers counts as you work.\n")
		return b.String()
	}
	pct := 0
	if w.Commands > 0 {
		pct = (w.Failures*100 + w.Commands/2) / w.Commands
	}
	fmt.Fprintf(&b, "  active on %d of %d days\n", w.ActiveDays, w.Days)
	fmt.Fprintf(&b, "  %s %s, %s %s (%d%%)\n", thousands(w.Commands), plural(w.Commands, "command", "commands"),
		thousands(w.Failures), plural(w.Failures, "failure", "failures"), pct)
	if w.LongestStreak >= 2 {
		fmt.Fprintf(&b, "  longest run of failures: %d, on %s\n", w.LongestStreak, w.LongestOn)
	}
	if w.LateNights > 0 {
		fmt.Fprintf(&b, "  %s with late-night work (after midnight): %d\n", plural(w.LateNights, "day", "days"), w.LateNights)
	}
	if w.Reliefs > 0 {
		fmt.Fprintf(&b, "  first greens after a rough patch: %d\n", w.Reliefs)
	}
	if w.Pushes > 0 {
		fmt.Fprintf(&b, "  pushes and deploys: %d\n", w.Pushes)
	}
	if w.ActiveDays > 1 {
		fmt.Fprintf(&b, "  busiest day: %s, %s commands\n", w.BusiestDay, thousands(w.BusiestCount))
	}
	if line := th.RecapLine(recapTone(w)); line != "" {
		fmt.Fprintf(&b, "\n  \"%s\"\n", line)
	}
	return b.String()
}

// runRecap handles `po recap [--days N]`.
func runRecap(args []string) {
	days := 7
	for i := 0; i < len(args); i++ {
		if args[i] == "--days" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 || n > memory.KeepDays {
				fmt.Fprintf(os.Stderr, "po: --days must be a number from 1 to %d\n", memory.KeepDays)
				os.Exit(2)
			}
			days = n
			i++
			continue
		}
		fmt.Fprintln(os.Stderr, "usage: po recap [--days N]")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}
	if cfg.Memory == "off" {
		fmt.Println("Memory is off, so there is nothing to recap. Turn it on with: po config set memory on")
		return
	}
	th, ok := theme.Get(cfg.Theme)
	if !ok {
		th = theme.Soccer
	}
	store := memory.Load(memory.DefaultPath())
	fmt.Print(formatRecap(store.Summary(time.Now(), days), th))
}

// runForget handles `po forget [--yes]`: it wipes what Po remembers.
func runForget(args []string) {
	yes := len(args) == 1 && args[0] == "--yes"
	if len(args) > 1 || (len(args) == 1 && !yes) {
		fmt.Fprintln(os.Stderr, "usage: po forget [--yes]")
		os.Exit(2)
	}
	path := memory.DefaultPath()
	fmt.Printf("This deletes everything Po remembers about your days (%s).\n", path)
	if !yes {
		if !isTerminal(os.Stdin) {
			fmt.Println("Nothing was deleted. Run po forget in a terminal to confirm, or add --yes.")
			return
		}
		if !ask(bufio.NewReader(os.Stdin), os.Stdout, "Delete it?", false) {
			fmt.Println("Nothing was deleted.")
			return
		}
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}
	ipc.Send(ipc.Message{Op: "forget"}) // a running Po forgets what it holds too
	fmt.Println("Forgotten.")
}
