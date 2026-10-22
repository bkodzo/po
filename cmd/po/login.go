package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/bkodzo/po/internal/ipc"
)

const defaultLoginLabel = "com.bkodzo.po.window"

// loginLabel names the login agent. PO_LOGIN_LABEL overrides it, so a test can
// use its own agent without touching a real one.
func loginLabel() string {
	if l := os.Getenv("PO_LOGIN_LABEL"); l != "" {
		return l
	}
	return defaultLoginLabel
}

// agentsDir is where macOS looks for per-user login agents. PO_LAUNCHAGENTS_DIR
// overrides it for tests.
func agentsDir() string {
	if d := os.Getenv("PO_LAUNCHAGENTS_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents")
}

func plistPath() string { return filepath.Join(agentsDir(), loginLabel()+".plist") }

func loginInstalled() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// passedThrough are the settings baked into the agent when they are set, so the
// agent runs the same Po that the command line does.
var passedThrough = []string{"PO_SOCK", "PO_CONFIG", "PO_WINDOW", "PO_NO_AUTOSTART"}

// buildPlist describes the login agent. It runs `po window run` at login, and
// starts it again only if it ends abnormally, so quitting it on purpose sticks
// until the next login.
func buildPlist(label, exe, logFile string, env map[string]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)
	fmt.Fprintf(&b, "\t<key>Label</key>\n\t<string>%s</string>\n", xmlEscape(label))
	fmt.Fprintf(&b, "\t<key>ProgramArguments</key>\n\t<array>\n\t\t<string>%s</string>\n\t\t<string>window</string>\n\t\t<string>run</string>\n\t</array>\n", xmlEscape(exe))
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	b.WriteString("\t<key>KeepAlive</key>\n\t<dict>\n\t\t<key>SuccessfulExit</key>\n\t\t<false/>\n\t</dict>\n")
	b.WriteString("\t<key>ThrottleInterval</key>\n\t<integer>10</integer>\n")
	fmt.Fprintf(&b, "\t<key>StandardOutPath</key>\n\t<string>%s</string>\n", xmlEscape(logFile))
	fmt.Fprintf(&b, "\t<key>StandardErrorPath</key>\n\t<string>%s</string>\n", xmlEscape(logFile))
	if len(env) > 0 {
		b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
		for _, k := range passedThrough {
			if v, ok := env[k]; ok {
				fmt.Fprintf(&b, "\t\t<key>%s</key>\n\t\t<string>%s</string>\n", k, xmlEscape(v))
			}
		}
		b.WriteString("\t</dict>\n")
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.String()
}

func launchctl(args ...string) (string, error) {
	out, err := exec.Command("launchctl", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func guiDomain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

var agentPIDPattern = regexp.MustCompile(`"PID"\s*=\s*(\d+);`)

// agentPID is the pid of the window when the login agent is the one running it.
func agentPID() (int, bool) {
	if !loginInstalled() {
		return 0, false
	}
	out, err := launchctl("list", loginLabel())
	if err != nil {
		return 0, false
	}
	m := agentPIDPattern.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	pid, err := strconv.Atoi(m[1])
	return pid, err == nil && pid > 1
}

func loginOn() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	// Don't install an agent that would have nothing to run.
	if _, err := findWindowProgram(); err != nil {
		return "", err
	}
	_, logFile, err := windowFiles()
	if err != nil {
		return "", err
	}
	env := map[string]string{}
	for _, k := range passedThrough {
		if v := os.Getenv(k); v != "" {
			env[k] = v
		}
	}
	if err := os.MkdirAll(agentsDir(), 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(plistPath(), []byte(buildPlist(loginLabel(), exe, logFile, env)), 0o644); err != nil {
		return "", err
	}

	// If the window was started by hand, let the agent take over so there is
	// only ever one.
	windowStop()
	launchctl("bootout", guiDomain()+"/"+loginLabel()) // fine if it was not loaded
	if out, err := launchctl("bootstrap", guiDomain(), plistPath()); err != nil {
		os.Remove(plistPath())
		return "", fmt.Errorf("launchctl could not load the login agent: %s", out)
	}
	return "the window now starts at every login (and has started now); turn it off with: po window login off", nil
}

func loginOff() string {
	launchctl("bootout", guiDomain()+"/"+loginLabel())
	removed := os.Remove(plistPath()) == nil
	windowStop()
	if !removed {
		return "the window was not set to start at login"
	}
	return "the window no longer starts at login (and has been closed); start it any time with: po window"
}

func loginStatus() string {
	if loginInstalled() {
		return "starts at login: yes"
	}
	return "starts at login: no (turn on with: po window login on)"
}

// runWindowForeground is what the login agent runs. It makes sure Po is up,
// unless it was stopped on purpose, and then becomes the window itself, so the
// agent supervises the window directly.
func runWindowForeground() error {
	program, err := findWindowProgram()
	if err != nil {
		return err
	}
	if ipc.Send(ipc.Message{Op: "ping"}) != nil && !stopped() {
		startDaemon()
	}
	return syscall.Exec(program, []string{program}, os.Environ())
}
