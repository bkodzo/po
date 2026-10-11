package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bkodzo/po/internal/event"
	"github.com/bkodzo/po/internal/ipc"
)

const zshHook = `# Po shell hook: tells Po how your commands went and where to show up.
zmodload zsh/datetime 2>/dev/null
autoload -Uz add-zsh-hook
_po_preexec() {
  _po_cmd=$1
  _po_start=$EPOCHREALTIME
  command po notify --hide --tty "$TTY" >/dev/null 2>&1
}
_po_precmd() {
  local code=$?
  local ms=0
  if [[ -n $_po_cmd ]]; then
    ms=$(( (EPOCHREALTIME - _po_start) * 1000 ))
    command po notify --status $code --ms ${ms%.*} --tty "$TTY" -- "$_po_cmd" >/dev/null 2>&1 &!
  else
    command po notify --tty "$TTY" >/dev/null 2>&1 &!
  fi
  _po_cmd=
}
_po_zshexit() { command po notify --hide --tty "$TTY" >/dev/null 2>&1 }
add-zsh-hook preexec _po_preexec
add-zsh-hook precmd _po_precmd
add-zsh-hook zshexit _po_zshexit
`

func runHook(args []string) {
	if len(args) != 1 || args[0] != "zsh" {
		fmt.Fprintln(os.Stderr, "usage: po hook zsh")
		os.Exit(2)
	}
	fmt.Print(zshHook)
}

// runNotify reports to a running pet, starting one in the background if
// needed. It never prints and always exits 0, so it can't disturb the shell it
// is called from.
func runNotify(args []string) {
	fs := flag.NewFlagSet("notify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	status := fs.Int("status", 0, "exit status of the command")
	ms := fs.Int64("ms", 0, "how long it ran, in milliseconds")
	tty := fs.String("tty", "", "terminal the shell is running in")
	hide := fs.Bool("hide", false, "hide the pet in that terminal")
	if fs.Parse(args) != nil {
		return
	}

	if *hide {
		_ = ipc.Send(ipc.Message{Op: "hide", TTY: *tty})
		return
	}

	msg := ipc.Message{TTY: *tty}
	if text := strings.TrimSpace(strings.Join(fs.Args(), " ")); text != "" {
		if c := (event.Command{Text: text, Exit: *status, Millis: *ms}); !c.Ignore() {
			msg.Cmd = &c
		}
	}
	if msg.Cmd == nil && msg.TTY == "" {
		return
	}
	if ipc.Send(msg) == nil {
		return
	}
	if os.Getenv("PO_NO_AUTOSTART") != "" || *tty == "" || stopped() || startDaemon() != nil {
		return
	}
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if ipc.Send(msg) == nil {
			return
		}
	}
}

// startDaemon launches the background pet, detached from this shell.
func startDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// offFile exists while Po has been stopped on purpose. Without it the hook
// would just start Po again at the next prompt.
func offFile() (string, error) {
	sock, err := ipc.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(sock), "off"), nil
}

func stopped() bool {
	path, err := offFile()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func runStop() {
	if path, err := offFile(); err == nil {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, nil, 0o644)
	}
	if err := ipc.Send(ipc.Message{Op: "stop"}); err != nil {
		fmt.Println("po is not running (it stays off until you run `po start`)")
		return
	}
	fmt.Println("po stopped (run `po start` to bring it back)")
}

func runStart() {
	if path, err := offFile(); err == nil {
		os.Remove(path)
	}
	if ipc.Send(ipc.Message{Op: "ping"}) == nil {
		fmt.Println("po is already running")
		return
	}
	if err := startDaemon(); err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}
	fmt.Println("po started; it appears at your next prompt")
}
