package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bkodzo/critter/internal/mood"
	"github.com/bkodzo/critter/internal/pet"
	"github.com/bkodzo/critter/internal/theme"
)

func main() {
	args := os.Args[1:]
	compare := len(args) > 0 && args[0] == "compare"
	if compare {
		args = args[1:]
	}

	fs := flag.NewFlagSet("critter", flag.ExitOnError)
	themeName := fs.String("theme", "soccer", "pet theme")
	readerName := fs.String("reader", "auto", "mood reader: auto, decision, ollama or keyword")
	url := fs.String("url", "", "address of the decision server or ollama")
	model := fs.String("model", "", "model name for the chosen reader")
	fs.Parse(args)

	keyword := mood.Keyword{}
	var remote mood.Reader
	if *readerName == "ollama" {
		remote = mood.NewOllama(*url, *model)
	} else {
		remote = mood.NewDecision(*url, *model)
	}

	if compare {
		runCompare(keyword, remote)
		return
	}

	th, ok := theme.Get(*themeName)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown theme %q\n", *themeName)
		os.Exit(1)
	}

	var reader mood.Reader
	switch *readerName {
	case "keyword":
		reader = keyword
	case "decision", "ollama":
		reader = remote
	case "auto":
		reader = mood.Fallback{Primary: remote, Backup: keyword}
	default:
		fmt.Fprintf(os.Stderr, "unknown reader %q\n", *readerName)
		os.Exit(1)
	}

	p := pet.Load()
	show(th, p, "")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		r, err := reader.Read(mood.Event{Kind: "text", Text: scanner.Text()})
		line := ""
		if err == nil && p.Apply(r) {
			line = th.Say(p.Mood)
		} else {
			line = th.Confused()
		}
		show(th, p, line)
		if err := p.Save(); err != nil {
			fmt.Fprintln(os.Stderr, "could not save state:", err)
		}
	}
}

func runCompare(keyword, local mood.Reader) {
	fmt.Printf("%-40s  %-18s  %s\n", "line", "keyword", "model")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		e := mood.Event{Kind: "text", Text: text}
		kw := describe(keyword, e)
		start := time.Now()
		md := describe(local, e)
		fmt.Printf("%-40.40s  %-18s  %s  (%dms)\n", text, kw, md, time.Since(start).Milliseconds())
	}
}

func describe(r mood.Reader, e mood.Event) string {
	reading, err := r.Read(e)
	if err != nil {
		return "error: " + err.Error()
	}
	m, c := reading.Top()
	return fmt.Sprintf("%s %.2f", m, c)
}

func show(th theme.Theme, p *pet.Pet, line string) {
	fmt.Printf("\n%s\n", th.Art[p.Mood])
	if line != "" {
		fmt.Printf("  \"%s\"\n", line)
	}
}
