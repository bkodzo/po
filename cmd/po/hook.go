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

// bashHook has no preexec to lean on, and bash's DEBUG trap clashes with other
// tools, so it reads the last history entry at each prompt and spots a new
// command by its history number. That also means a command typed with a leading
// space, which bash keeps out of history under the usual settings, stays
// invisible to Po. It cannot know how long a command took, and it cannot hide
// the corner pet while one runs, so it asks Po not to draw it. It avoids
// anything newer than bash 3.2, the version macOS ships.
const bashHook = `# Po shell hook for bash: tells Po how your commands went.
if [ -z "${_po_hooked:-}" ]; then
  _po_hooked=1
  _po_tty=$(tty 2>/dev/null)
  _po_last=
  _po_history_number() {
    local line
    line=$(HISTTIMEFORMAT= history 1 2>/dev/null)
    line=${line#"${line%%[![:space:]]*}"}
    printf '%s' "${line%%[[:space:]]*}"
  }
  _po_last=$(_po_history_number)
  _po_precmd() {
    local code=$? line n cmd
    line=$(HISTTIMEFORMAT= history 1 2>/dev/null)
    line=${line#"${line%%[![:space:]]*}"}
    n=${line%%[[:space:]]*}
    cmd=${line#"$n"}
    cmd=${cmd#"${cmd%%[![:space:]]*}"}
    if [ -n "$n" ] && [ "$n" != "$_po_last" ] && [ -n "$cmd" ]; then
      _po_last=$n
      ( command po notify --no-overlay --status "$code" --ms 0 --tty "$_po_tty" -- "$cmd" >/dev/null 2>&1 & )
    else
      ( command po notify --no-overlay --tty "$_po_tty" >/dev/null 2>&1 & )
    fi
    return "$code"
  }
  case "$(declare -p PROMPT_COMMAND 2>/dev/null)" in
    "declare -a"*) PROMPT_COMMAND=(_po_precmd "${PROMPT_COMMAND[@]}") ;;
    *) case ";${PROMPT_COMMAND:-};" in
         *";_po_precmd;"*) ;;
         *) PROMPT_COMMAND="_po_precmd${PROMPT_COMMAND:+;$PROMPT_COMMAND}" ;;
       esac ;;
  esac
fi
`

// fishHook uses fish's own events, which also supply the command's duration.
const fishHook = `# Po shell hook for fish: tells Po how your commands went and where to show up.
if not set -q __po_hooked
    set -g __po_hooked 1
    set -g __po_tty (tty 2>/dev/null)

    function __po_preexec --on-event fish_preexec
        command po notify --hide --tty "$__po_tty" >/dev/null 2>&1
    end

    function __po_postexec --on-event fish_postexec
        set -l code $status
        command po notify --status $code --ms $CMD_DURATION --tty "$__po_tty" -- "$argv" >/dev/null 2>&1 &
        disown 2>/dev/null
    end

    # The first prompt of a new shell: show up, and start Po if it is not running.
    function __po_first_prompt --on-event fish_prompt
        functions -e __po_first_prompt
        command po notify --tty "$__po_tty" >/dev/null 2>&1 &
        disown 2>/dev/null
    end
end
`

// hookScript returns the hook for a shell, or false for an unknown one.
func hookScript(shell string) (string, bool) {
	switch shell {
	case "zsh":
		return zshHook, true
	case "bash":
		return bashHook, true
	case "fish":
		return fishHook, true
	}
	return "", false
}

func runHook(args []string) {
	script, ok := hookScript("")
	if len(args) == 1 {
		script, ok = hookScript(args[0])
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: po hook zsh|bash|fish")
		os.Exit(2)
	}
	fmt.Print(script)
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
	noOverlay := fs.Bool("no-overlay", false, "do not draw the corner pet in that terminal")
	if fs.Parse(args) != nil {
		return
	}

	if *hide {
		_ = ipc.Send(ipc.Message{Op: "hide", TTY: *tty})
		return
	}

	msg := ipc.Message{TTY: *tty, NoOverlay: *noOverlay}
	msg.Cmd = commandFrom(fs.Args(), *status, *ms)
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

// commandFrom builds the command a shell hook reports, or nil if there is
// nothing to send.
//
// A command typed with a leading space is private by shell convention, and the
// space is the only sign of it, so it must be checked before any trimming. A
// private command is not sent at all: Po never hears of it. Everything else is
// sent, even the dull commands, because Po decides what to react to; they still
// count as someone being at the keyboard.
func commandFrom(args []string, status int, ms int64) *event.Command {
	text := strings.TrimRight(strings.Join(args, " "), " \t\r\n")
	c := event.Command{Text: text, Exit: status, Millis: ms}
	if c.Private() {
		return nil
	}
	return &c
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

// stopPo stops Po and keeps it off until `po start`.
func stopPo() string {
	if path, err := offFile(); err == nil {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, nil, 0o644)
	}
	if err := ipc.Send(ipc.Message{Op: "stop"}); err != nil {
		return "po is not running (it stays off until you run `po start`)"
	}
	return "po stopped (run `po start` to bring it back)"
}

func runStop() { fmt.Println(stopPo()) }

// startPo clears the "stopped on purpose" flag and starts Po in the background
// if it is not already running.
func startPo() (string, error) {
	if path, err := offFile(); err == nil {
		os.Remove(path)
	}
	if ipc.Send(ipc.Message{Op: "ping"}) == nil {
		return "po is already running", nil
	}
	if err := startDaemon(); err != nil {
		return "", err
	}
	return "po started; it appears at your next prompt", nil
}

func runStart() {
	msg, err := startPo()
	if err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}
	fmt.Println(msg)
}
