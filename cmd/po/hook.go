package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bkodzo/po/internal/event"
	"github.com/bkodzo/po/internal/ipc"
)

const zshHook = `# Po shell hook: tells Po how your commands went.
zmodload zsh/datetime 2>/dev/null
autoload -Uz add-zsh-hook
_po_preexec() { _po_cmd=$1; _po_start=$EPOCHREALTIME }
_po_precmd() {
  local code=$?
  [[ -n $_po_cmd ]] || return
  local ms=$(( (EPOCHREALTIME - _po_start) * 1000 ))
  command po notify --status $code --ms ${ms%.*} -- "$_po_cmd" >/dev/null 2>&1 &!
  _po_cmd=
}
add-zsh-hook preexec _po_preexec
add-zsh-hook precmd _po_precmd
`

func runHook(args []string) {
	if len(args) != 1 || args[0] != "zsh" {
		fmt.Fprintln(os.Stderr, "usage: po hook zsh")
		os.Exit(2)
	}
	fmt.Print(zshHook)
}

// runNotify reports one finished command to a running pet. It never prints and
// always exits 0, so it can't disturb the shell it is called from.
func runNotify(args []string) {
	fs := flag.NewFlagSet("notify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	status := fs.Int("status", 0, "exit status of the command")
	ms := fs.Int64("ms", 0, "how long it ran, in milliseconds")
	if fs.Parse(args) != nil {
		return
	}
	text := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if event.Skip(text) {
		return
	}
	_ = ipc.Send(event.Command{Text: text, Exit: *status, Millis: *ms})
}
