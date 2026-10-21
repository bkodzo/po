package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bkodzo/po/internal/ipc"
)

const windowProgram = "po-window"

// windowFiles returns where the window's pid and log are kept, beside the
// socket, so a test or a second setup with its own socket gets its own window.
func windowFiles() (pidFile, logFile string, err error) {
	sock, err := ipc.Path()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Dir(sock)
	return filepath.Join(dir, "window.pid"), filepath.Join(dir, "window.log"), nil
}

// findWindowProgram looks for the desktop window program: PO_WINDOW, then next
// to this program, then in the repository's build folder.
func findWindowProgram() (string, error) {
	if p := os.Getenv("PO_WINDOW"); p != "" {
		return p, nil
	}
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		for _, c := range []string{
			filepath.Join(dir, windowProgram),
			filepath.Join(dir, "desktop", ".build", "release", windowProgram),
		} {
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				return c, nil
			}
		}
	}
	return "", errors.New(windowProgram + " not found; build and install it with desktop/install.sh")
}

// runningWindow returns the pid of the window if it is running. The command
// name is checked too, so a recycled pid is never mistaken for it.
func runningWindow() (int, bool) {
	pidFile, _, err := windowFiles()
	if err != nil {
		return 0, false
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 || syscall.Kill(pid, 0) != nil {
		return 0, false
	}
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil || !strings.Contains(string(out), windowProgram) {
		return 0, false
	}
	return pid, true
}

func windowStart() (string, error) {
	if pid, ok := runningWindow(); ok {
		return fmt.Sprintf("the window is already running (pid %d)", pid), nil
	}
	program, err := findWindowProgram()
	if err != nil {
		return "", err
	}
	pidFile, logFile, err := windowFiles()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(pidFile), 0o755); err != nil {
		return "", err
	}
	log, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	defer log.Close()

	cmd := exec.Command(program)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	pid := cmd.Process.Pid
	cmd.Process.Release()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		return "", err
	}

	// The window waits for Po, so make sure Po is there unless it was stopped
	// on purpose.
	note := ""
	if ipc.Send(ipc.Message{Op: "ping"}) != nil {
		if stopped() {
			note = " (Po is stopped on purpose, so the pet stays hidden until you run po start)"
		} else if startDaemon() == nil {
			note = " (started Po too)"
		}
	}
	return fmt.Sprintf("window started (pid %d)%s", pid, note), nil
}

func windowStop() string {
	pidFile, _, _ := windowFiles()
	pid, ok := runningWindow()
	os.Remove(pidFile)
	if !ok {
		return "the window is not running"
	}
	syscall.Kill(pid, syscall.SIGTERM)
	for i := 0; i < 30; i++ {
		if syscall.Kill(pid, 0) != nil {
			return "window stopped"
		}
		time.Sleep(100 * time.Millisecond)
	}
	syscall.Kill(pid, syscall.SIGKILL)
	return "window stopped (it had to be forced)"
}

func windowStatus() string {
	if pid, ok := runningWindow(); ok {
		return fmt.Sprintf("the window is running (pid %d)", pid)
	}
	return "the window is not running (start it with: po window)"
}

func runWindow(args []string) {
	sub := "start"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "start":
		msg, err := windowStart()
		if err != nil {
			fmt.Fprintln(os.Stderr, "po:", err)
			os.Exit(1)
		}
		fmt.Println(msg)
	case "stop":
		fmt.Println(windowStop())
	case "status":
		fmt.Println(windowStatus())
	default:
		fmt.Fprintln(os.Stderr, "usage: po window [start | stop | status]")
		os.Exit(2)
	}
}
