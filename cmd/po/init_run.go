package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// runInit handles `po init [--yes] [--undo]`.
func runInit(args []string) {
	yes, undo, shell := false, false, ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--yes", "-y":
			yes = true
		case "--undo":
			undo = true
		case "--shell":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "usage: po init [--yes] [--undo] [--shell zsh|bash|fish]")
				os.Exit(2)
			}
			shell = args[i+1]
			i++
		default:
			fmt.Fprintln(os.Stderr, "usage: po init [--yes] [--undo] [--shell zsh|bash|fish]")
			os.Exit(2)
		}
	}
	if shell == "" {
		shell = filepath.Base(os.Getenv("SHELL"))
	}
	if _, ok := hookScript(shell); !ok {
		fmt.Printf("Your shell is %q. Po can watch zsh, bash and fish. Choose one with: po init --shell zsh|bash|fish\n", shell)
		fmt.Println("Nothing was changed.")
		return
	}

	exe, _ := os.Executable()
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	home, _ := os.UserHomeDir()
	env := initEnv{
		in:          os.Stdin,
		out:         os.Stdout,
		rcPath:      rcPathFor(shell, home),
		shell:       shell,
		ownFile:     shell == "fish",
		home:        home,
		exeDir:      filepath.Dir(exe),
		onPath:      func() bool { _, err := exec.LookPath("po"); return err == nil },
		interactive: isTerminal(os.Stdin),
		yes:         yes,

		startPo:         startPo,
		windowAvailable: func() bool { _, err := findWindowProgram(); return err == nil },
		startWindow:     windowStart,
		loginOn:         loginOn,
		stopAll: func() []string {
			var lines []string
			if loginInstalled() {
				lines = append(lines, loginOff())
			} else {
				lines = append(lines, windowStop())
			}
			return append(lines, stopPo())
		},
	}

	var err error
	if undo {
		err = undoInit(env)
	} else {
		err = runInitWith(env)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "po:", err)
		os.Exit(1)
	}
}

// rcPathFor is the startup file Po's hook goes in for a shell.
func rcPathFor(shell, home string) string {
	switch shell {
	case "bash":
		return bashRcPath(home, runtime.GOOS, func(p string) bool { _, err := os.Stat(p); return err == nil })
	case "fish":
		return fishConfPath(home, os.Getenv("XDG_CONFIG_HOME"))
	}
	return zshrcPath()
}
