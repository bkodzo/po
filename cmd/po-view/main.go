// po-view is the full-size window for Po. It lives in its own program because
// the terminal UI library it uses probes the terminal when it starts, which the
// everyday po command should never do.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/bkodzo/po/internal/config"
	"github.com/bkodzo/po/internal/ipc"
	"github.com/bkodzo/po/internal/pet"
	"github.com/bkodzo/po/internal/stack"
	"github.com/bkodzo/po/internal/theme"
	"github.com/bkodzo/po/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	flag.StringVar(&cfg.Theme, "theme", cfg.Theme, "pet theme")
	flag.StringVar(&cfg.Reader, "reader", cfg.Reader, "mood reader: auto, rules, keyword, decision or ollama")
	flag.StringVar(&cfg.URL, "url", cfg.URL, "address of the decision server or ollama")
	flag.StringVar(&cfg.Model, "model", cfg.Model, "model name")
	flag.Parse()

	th, ok := theme.Get(cfg.Theme)
	if !ok {
		fatal(fmt.Errorf("unknown theme %q", cfg.Theme))
	}
	st := stack.Build(cfg)
	p := pet.Load()
	p.MinConfidence = st.MinConfidence

	listener, err := ipc.Listen()
	if errors.Is(err, ipc.ErrRunning) {
		fatal(errors.New("Po is already running in the background (run `po stop` first)"))
	}
	if err != nil {
		fatal(err)
	}
	defer listener.Close()

	if err := ui.Run(th, p, st.Reader, listener.C); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "po-view:", err)
	os.Exit(1)
}
