package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The lines `po init` adds to the shell config. Each block starts with a marker
// comment, so `po init --undo` removes exactly these and nothing else.
const (
	hookMarker = "# Po: reacts to your shell commands (added by po init)"
	hookLine   = `eval "$(po hook zsh)"`
	pathMarker = "# Po: makes the po command available (added by po init)"
)

// zshrcPath is the file zsh reads when it starts an interactive shell.
func zshrcPath() string {
	if dir := os.Getenv("ZDOTDIR"); dir != "" {
		return filepath.Join(dir, ".zshrc")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".zshrc")
}

// hasHook reports whether the shell config already loads Po's hook, however it
// got there. A commented-out line does not count.
func hasHook(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "#") && strings.Contains(t, "po hook zsh") {
			return true
		}
	}
	return false
}

// pathExport is the line that puts dir on the PATH, written with $HOME where
// possible so the file still works if the home folder is renamed.
func pathExport(dir, home string) string {
	if home != "" && (dir == home || strings.HasPrefix(dir, home+"/")) {
		dir = "$HOME" + strings.TrimPrefix(dir, home)
	}
	return fmt.Sprintf(`export PATH="%s:$PATH"`, dir)
}

func hasPathExport(content, exportLine string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == exportLine {
			return true
		}
	}
	return false
}

// withBlock returns content with a marked block added at the end.
func withBlock(content, marker, line string) string {
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + "\n" + marker + "\n" + line + "\n"
}

// withoutBlock removes a block added by withBlock. It reports whether it found
// one. Only the marker and the line right after it are removed, along with the
// blank line withBlock put before them.
func withoutBlock(content, marker string) (string, bool) {
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		if l != marker || i+1 >= len(lines) {
			continue
		}
		start, end := i, i+2 // the marker and the line after it
		if start > 0 && lines[start-1] == "" {
			start--
		}
		return strings.Join(append(lines[:start:start], lines[end:]...), "\n"), true
	}
	return content, false
}

// ask puts a yes/no question and returns the answer. Pressing Enter takes the
// default, but input that ends without any answer counts as no.
func ask(in *bufio.Reader, out io.Writer, question string, def bool) bool {
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	fmt.Fprintf(out, "%s %s ", question, hint)
	line, err := in.ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		// The input ended without an answer. That is not consent, whatever the
		// default is, so nothing is done.
		fmt.Fprintln(out)
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	}
	return def
}

// initEnv is everything `po init` touches, so tests can stand in for it.
type initEnv struct {
	in          io.Reader
	out         io.Writer
	rcPath      string
	home        string
	exeDir      string // the folder this program is in
	onPath      func() bool
	interactive bool
	yes         bool

	startPo         func() (string, error)
	windowAvailable func() bool
	startWindow     func() (string, error)
	loginOn         func() (string, error)
	stopAll         func() []string
}

// backupOnce copies path to path+suffix the first time only, so the original
// is never overwritten by a later copy.
func backupOnce(path, suffix string) (string, error) {
	backup := path + suffix
	if _, err := os.Stat(backup); err == nil {
		return backup, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil // nothing to back up
	}
	if err != nil {
		return "", err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return backup, os.WriteFile(backup, data, mode)
}

func writeKeepingMode(path, content string) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), mode)
}

func runInitWith(e initEnv) error {
	out := e.out
	in := bufio.NewReader(e.in)
	data, err := os.ReadFile(e.rcPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)

	needHook := !hasHook(content)
	pathLine := pathExport(e.exeDir, e.home)
	needPath := !e.onPath() && !hasPathExport(content, pathLine)

	fmt.Fprintln(out, "Po setup")
	fmt.Fprintln(out)
	if !needHook && !needPath {
		fmt.Fprintf(out, "Your shell config (%s) already loads Po, so it needs no change.\n", e.rcPath)
	} else {
		fmt.Fprintf(out, "I will add this to %s (a backup is saved first):\n\n", e.rcPath)
		if needPath {
			fmt.Fprintf(out, "    %s\n", pathLine)
		}
		if needHook {
			fmt.Fprintf(out, "    %s\n", hookLine)
		}
		fmt.Fprintln(out)
	}

	// Without a terminal to answer, only describe the plan.
	if !e.interactive && !e.yes {
		fmt.Fprintln(out, "Nothing was changed. Run po init in a terminal to answer the questions, or add --yes.")
		return nil
	}

	if (needHook || needPath) && !e.yes && !ask(in, out, "Make this change?", true) {
		fmt.Fprintln(out, "No changes made.")
		return nil
	}

	if needHook || needPath {
		backup, err := backupOnce(e.rcPath, ".po-backup")
		if err != nil {
			return fmt.Errorf("could not back up %s: %w", e.rcPath, err)
		}
		if needPath {
			content = withBlock(content, pathMarker, pathLine)
		}
		if needHook {
			content = withBlock(content, hookMarker, hookLine)
		}
		if err := writeKeepingMode(e.rcPath, content); err != nil {
			return err
		}
		if backup != "" {
			fmt.Fprintf(out, "Added it. Your original is saved as %s\n", backup)
		} else {
			fmt.Fprintf(out, "Created %s with it.\n", e.rcPath)
		}
	}

	msg, err := e.startPo()
	if err != nil {
		fmt.Fprintf(out, "Could not start Po: %v\n", err)
	} else {
		fmt.Fprintln(out, msg+".")
	}

	if e.windowAvailable() {
		if e.yes || ask(in, out, "Show the desktop pet now?", true) {
			if msg, err := e.startWindow(); err != nil {
				fmt.Fprintf(out, "Could not start the desktop pet: %v\n", err)
			} else {
				fmt.Fprintln(out, msg+".")
				if !e.yes && ask(in, out, "Start it automatically at every login?", false) {
					if msg, err := e.loginOn(); err != nil {
						fmt.Fprintf(out, "Could not set that up: %v\n", err)
					} else {
						fmt.Fprintln(out, msg)
					}
				}
			}
		}
	} else {
		fmt.Fprintln(out, "The desktop pet program (po-window) is not installed; build it with desktop/install.sh.")
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Open a new terminal tab to start using it. Po reacts to your commands from the next prompt.")
	fmt.Fprintln(out, "It works with no model at all. To use one, see: po config")
	fmt.Fprintln(out, "To undo everything this did: po init --undo")
	return nil
}

// undoInit removes what `po init` added and stops Po and its window.
func undoInit(e initEnv) error {
	out := e.out
	data, err := os.ReadFile(e.rcPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)
	var removed []string
	if c, ok := withoutBlock(content, hookMarker); ok {
		content, removed = c, append(removed, "the Po hook")
	}
	if c, ok := withoutBlock(content, pathMarker); ok {
		content, removed = c, append(removed, "the PATH line")
	}
	if len(removed) > 0 {
		if _, err := backupOnce(e.rcPath, ".po-undo-backup"); err != nil {
			return err
		}
		if err := writeKeepingMode(e.rcPath, content); err != nil {
			return err
		}
		fmt.Fprintf(out, "Removed %s from %s.\n", strings.Join(removed, " and "), e.rcPath)
	} else if hasHook(content) {
		fmt.Fprintf(out, "%s loads the Po hook, but not from a block po init added, so I left it alone. Delete that line yourself to remove it.\n", e.rcPath)
	} else {
		fmt.Fprintf(out, "Nothing to remove from %s.\n", e.rcPath)
	}
	for _, line := range e.stopAll() {
		fmt.Fprintln(out, line)
	}
	return nil
}
