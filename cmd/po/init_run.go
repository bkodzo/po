package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// runInit handles `po init [--yes] [--undo]`.
func runInit(args []string) {
	yes, undo := false, false
	for _, a := range args {
		switch a {
		case "--yes", "-y":
			yes = true
		case "--undo":
			undo = true
		default:
			fmt.Fprintln(os.Stderr, "usage: po init [--yes] [--undo]")
			os.Exit(2)
		}
	}

	exe, _ := os.Executable()
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	home, _ := os.UserHomeDir()
	env := initEnv{
		in:          os.Stdin,
		out:         os.Stdout,
		rcPath:      zshrcPath(),
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

	if filepath.Base(os.Getenv("SHELL")) != "zsh" && os.Getenv("ZDOTDIR") == "" {
		fmt.Printf("Your shell is %s. Po's hook supports zsh only for now, so Po would not see your commands.\n", os.Getenv("SHELL"))
		fmt.Println("Nothing was changed.")
		return
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
