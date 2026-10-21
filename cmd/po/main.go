package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bkodzo/po/internal/config"
	"github.com/bkodzo/po/internal/daemon"
	"github.com/bkodzo/po/internal/ipc"
	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/pet"
	"github.com/bkodzo/po/internal/stack"
	"github.com/bkodzo/po/internal/theme"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "notify":
			runNotify(args[1:])
			return
		case "hook":
			runHook(args[1:])
			return
		case "stop":
			runStop()
			return
		case "start":
			runStart()
			return
		case "status":
			runStatus()
			return
		case "config":
			runConfig(args[1:])
			return
		case "watch":
			runWatch()
			return
		case "theme":
			runTheme(args[1:])
			return
		}
	}

	sub := ""
	if len(args) > 0 && (args[0] == "compare" || args[0] == "eval" || args[0] == "gallery" || args[0] == "daemon") {
		sub, args = args[0], args[1:]
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}

	fs := flag.NewFlagSet("po", flag.ExitOnError)
	themeName := fs.String("theme", cfg.Theme, "pet theme")
	readerName := fs.String("reader", cfg.Reader, "mood reader: auto, rules, keyword, decision or ollama")
	url := fs.String("url", cfg.URL, "address of the decision server or ollama")
	model := fs.String("model", cfg.Model, "model name")
	letters := fs.Bool("letters", false, "gallery: print plain letters instead of colour")
	data := fs.String("data", "testdata/lines.tsv", "labelled lines for eval")
	misses := fs.Bool("misses", false, "eval: list every line a reader got wrong")
	evalReaders := fs.String("readers", "keyword,rules", "eval: readers to score, from keyword, rules, model")
	fs.Parse(args)
	cfg.Theme, cfg.Reader, cfg.URL, cfg.Model = *themeName, *readerName, *url, *model

	th, ok := theme.Get(cfg.Theme)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown theme %q\n", cfg.Theme)
		os.Exit(1)
	}

	switch sub {
	case "compare":
		runCompare(mood.Keyword{}, modelReader(cfg))
		return
	case "gallery":
		gallery(th, *letters)
		return
	case "eval":
		showMisses = *misses
		readers := map[string]mood.Reader{"keyword": mood.Keyword{}, "rules": mood.Rules{}, "model": modelReader(cfg)}
		var order []string
		for _, name := range strings.Split(*evalReaders, ",") {
			name = strings.TrimSpace(name)
			if _, ok := readers[name]; !ok {
				fmt.Fprintf(os.Stderr, "unknown reader %q (use keyword, rules or model)\n", name)
				os.Exit(1)
			}
			order = append(order, name)
		}
		if err := runEval(*data, readers, order); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	st := stack.Build(cfg)
	p := pet.Load()
	p.MinConfidence = st.MinConfidence

	if sub == "daemon" {
		listener, err := ipc.Listen()
		if err != nil {
			os.Exit(1)
		}
		defer listener.Close()
		daemon.Run(th, p, st, listener)
		return
	}

	if isTerminal(os.Stdin) {
		fmt.Print(usage)
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		r, err := st.Reader.Read(mood.Event{Kind: "text", Text: scanner.Text()})
		line := ""
		if err == nil && p.Apply(r) {
			line = th.Say(p.Mood)
		} else {
			line = th.Confused()
		}
		show(p, line)
		if err := p.Save(); err != nil {
			fmt.Fprintln(os.Stderr, "could not save state:", err)
		}
	}
}

// modelReader is the model on its own, for comparing and scoring.
func modelReader(cfg config.Config) mood.Reader {
	if cfg.Reader == "ollama" {
		return mood.NewOllama(cfg.URL, cfg.Model)
	}
	d := mood.NewDecision(cfg.URL, cfg.Model)
	d.Key = cfg.ReadKey()
	return d
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

func show(p *pet.Pet, line string) {
	fmt.Printf("[%s] %s\n", p.Mood, line)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// gallery prints one representative frame for every mood.
func gallery(th theme.Theme, letters bool) {
	ticks := map[mood.Mood]int{mood.Celebrating: 3, mood.Frustrated: 1, mood.Tired: 9}
	for _, m := range mood.All {
		canvas, pal := th.Frame(m, ticks[m])
		fmt.Printf("\n%s\n", m)
		if letters {
			fmt.Println(canvas.Letters())
		} else {
			fmt.Println(canvas.Render(pal))
		}
	}
}

const usage = `po: a mood-reactive pet for your terminal

  po hook zsh      print the shell hook; add  eval "$(po hook zsh)"  to ~/.zshrc
  po start         start Po in the background (the hook also does this)
  po stop          stop Po; it stays off until you run po start
  po status        show what Po is doing, which reader it uses and where it sends data
  po config        show or change settings (reader, model address, API key source)
  po watch         print the pet's state as it changes (what a window receives)
  po theme export  write the theme as data for a window to play
  po gallery       show every mood
  po compare       compare the keyword reader with the model (lines from stdin)
  po eval          score the readers on the labelled lines in testdata/
  po-view          open the full-size window instead of the corner pet
`
