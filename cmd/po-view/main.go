// po-view is the full-size window for Po. It lives in its own program because
// the terminal UI library it uses probes the terminal when it starts, which the
// everyday po command should never do.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/bkodzo/po/internal/ipc"
	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/pet"
	"github.com/bkodzo/po/internal/theme"
	"github.com/bkodzo/po/internal/ui"
)

func main() {
	themeName := flag.String("theme", "soccer", "pet theme")
	readerName := flag.String("reader", "auto", "mood reader: auto, decision, ollama or keyword")
	url := flag.String("url", "", "address of the decision server or ollama")
	model := flag.String("model", "", "model name for the chosen reader")
	flag.Parse()

	th, ok := theme.Get(*themeName)
	if !ok {
		fatal(fmt.Errorf("unknown theme %q", *themeName))
	}
	reader, err := mood.NewReader(*readerName, *url, *model)
	if err != nil {
		fatal(err)
	}

	p := pet.Load()
	if *readerName != "keyword" {
		p.MinConfidence = 0.25
	}

	listener, err := ipc.Listen()
	if errors.Is(err, ipc.ErrRunning) {
		fatal(errors.New("Po is already running in the background (run `po stop` first)"))
	}
	if err != nil {
		fatal(err)
	}
	defer listener.Close()

	if err := ui.Run(th, p, reader, listener.C); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "po-view:", err)
	os.Exit(1)
}
