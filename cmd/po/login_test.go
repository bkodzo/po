package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlistIsValidAndHasTheRightBehaviour(t *testing.T) {
	plist := buildPlist("com.example.test", "/Users/me/go/bin/po", "/tmp/window.log",
		map[string]string{"PO_SOCK": "/tmp/a&b/s", "PO_CONFIG": "/tmp/cfg"})

	path := filepath.Join(t.TempDir(), "x.plist")
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("plutil"); err == nil {
		if out, err := exec.Command("plutil", "-lint", path).CombinedOutput(); err != nil {
			t.Fatalf("plutil rejects the plist: %s\n%s", out, plist)
		}
	}
	for _, want := range []string{
		"<string>com.example.test</string>",
		"<string>/Users/me/go/bin/po</string>",
		"<string>window</string>", "<string>run</string>",
		"<key>RunAtLoad</key>\n\t<true/>",
		"<key>SuccessfulExit</key>\n\t\t<false/>", // restart only after a crash
		"/tmp/a&amp;b/s",                          // special characters are escaped
	} {
		if !strings.Contains(plist, want) {
			t.Errorf("plist is missing %q:\n%s", want, plist)
		}
	}
}

func TestPlistWithoutSettingsHasNoEnvironment(t *testing.T) {
	plist := buildPlist("l", "/po", "/log", nil)
	if strings.Contains(plist, "EnvironmentVariables") {
		t.Error("no settings were passed, so no environment should be written")
	}
}

func TestLoginOnRefusesWhenThereIsNoWindowProgram(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PO_LAUNCHAGENTS_DIR", dir)
	t.Setenv("PO_LOGIN_LABEL", "com.example.noprogram")
	t.Setenv("PO_SOCK", filepath.Join(dir, "s"))
	t.Setenv("PO_WINDOW", "/no/such/po-window")
	if _, err := loginOn(); err == nil {
		t.Fatal("expected an error: there is no window program to run")
	}
	if loginInstalled() {
		t.Error("an agent that cannot run must not be installed")
	}
}

func TestLoginStatusFollowsThePlist(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PO_LAUNCHAGENTS_DIR", dir)
	t.Setenv("PO_LOGIN_LABEL", "com.example.statustest")
	if got := loginStatus(); !strings.Contains(got, "no") {
		t.Errorf("before: %q", got)
	}
	os.WriteFile(plistPath(), []byte("x"), 0o644)
	if got := loginStatus(); !strings.Contains(got, "yes") {
		t.Errorf("after: %q", got)
	}
}

func TestLoginOffWhenNeverOn(t *testing.T) {
	t.Setenv("PO_LAUNCHAGENTS_DIR", t.TempDir())
	t.Setenv("PO_LOGIN_LABEL", "com.example.neveron")
	t.Setenv("PO_SOCK", filepath.Join(t.TempDir(), "s"))
	if got := loginOff(); !strings.Contains(got, "was not set") {
		t.Errorf("got %q", got)
	}
}
