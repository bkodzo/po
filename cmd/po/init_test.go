package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeSteps struct {
	started, windowStarted, loginTurnedOn int
	stopped                               int
}

// env builds an initEnv whose side effects are counted, not performed.
func env(t *testing.T, rc string, answers string, interactive, yes bool) (initEnv, *fakeSteps, *bytes.Buffer) {
	t.Helper()
	steps := &fakeSteps{}
	out := &bytes.Buffer{}
	return initEnv{
		in:          strings.NewReader(answers),
		out:         out,
		rcPath:      rc,
		home:        "/Users/me",
		exeDir:      "/Users/me/go/bin",
		onPath:      func() bool { return true },
		interactive: interactive,
		yes:         yes,

		startPo:         func() (string, error) { steps.started++; return "po started", nil },
		windowAvailable: func() bool { return true },
		startWindow:     func() (string, error) { steps.windowStarted++; return "window started", nil },
		loginOn:         func() (string, error) { steps.loginTurnedOn++; return "login on", nil },
		stopAll:         func() []string { steps.stopped++; return []string{"stopped"} },
	}, steps, out
}

func rcFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".zshrc")
	if content != "\x00" { // "\x00" means no file at all
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInitAddsTheHookAndKeepsEverythingElse(t *testing.T) {
	original := "export EDITOR=vim\nalias ll='ls -l'\n"
	rc := rcFile(t, original)
	e, steps, _ := env(t, rc, "y\nn\n", true, false)
	if err := runInitWith(e); err != nil {
		t.Fatal(err)
	}
	got := read(t, rc)
	if !strings.HasPrefix(got, original) {
		t.Errorf("the original content was changed:\n%s", got)
	}
	if !strings.Contains(got, hookMarker+"\n"+hookLine+"\n") {
		t.Errorf("the hook block is missing:\n%s", got)
	}
	if read(t, rc+".po-backup") != original {
		t.Error("the backup is not the original")
	}
	if steps.started != 1 {
		t.Errorf("Po was started %d times, want 1", steps.started)
	}
}

func TestInitHandlesAFileWithNoFinalNewlineAndNoFileAtAll(t *testing.T) {
	rc := rcFile(t, "export A=1") // no trailing newline
	e, _, _ := env(t, rc, "y\nn\n", true, false)
	if err := runInitWith(e); err != nil {
		t.Fatal(err)
	}
	if got := read(t, rc); !strings.HasPrefix(got, "export A=1\n\n"+hookMarker) {
		t.Errorf("the last line was run into the block:\n%q", got)
	}

	missing := rcFile(t, "\x00")
	e, _, _ = env(t, missing, "y\nn\n", true, false)
	if err := runInitWith(e); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, missing), hookLine) {
		t.Error("a missing .zshrc should be created with the hook")
	}
	if _, err := os.Stat(missing + ".po-backup"); err == nil {
		t.Error("there was nothing to back up")
	}
}

func TestInitTwiceChangesNothingTheSecondTime(t *testing.T) {
	rc := rcFile(t, "export A=1\n")
	e, _, _ := env(t, rc, "y\nn\n", true, false)
	runInitWith(e)
	afterFirst := read(t, rc)

	e, _, out := env(t, rc, "n\n", true, false)
	if err := runInitWith(e); err != nil {
		t.Fatal(err)
	}
	if read(t, rc) != afterFirst {
		t.Error("the second run changed the file")
	}
	if !strings.Contains(out.String(), "already loads Po") {
		t.Errorf("it should say it is already set up:\n%s", out)
	}
	if strings.Count(read(t, rc), hookLine) != 1 {
		t.Error("the hook was added twice")
	}
}

func TestAHandWrittenHookIsRecognisedAndLeftAlone(t *testing.T) {
	original := "export A=1\neval \"$(po hook zsh)\"\n"
	rc := rcFile(t, original)
	e, _, _ := env(t, rc, "n\n", true, false)
	runInitWith(e)
	if read(t, rc) != original {
		t.Error("a hook the user wrote themselves must not be touched or duplicated")
	}
	if _, err := os.Stat(rc + ".po-backup"); err == nil {
		t.Error("no change was made, so no backup is needed")
	}
}

func TestACommentedOutHookDoesNotCount(t *testing.T) {
	if hasHook("# eval \"$(po hook zsh)\"\n") {
		t.Error("a commented-out hook is not a hook")
	}
	if !hasHook("  eval \"$(po hook zsh)\"  \n") {
		t.Error("an indented hook is a hook")
	}
}

func TestDecliningChangesNothing(t *testing.T) {
	original := "export A=1\n"
	rc := rcFile(t, original)
	e, steps, out := env(t, rc, "n\n", true, false)
	if err := runInitWith(e); err != nil {
		t.Fatal(err)
	}
	if read(t, rc) != original {
		t.Error("the file changed although the answer was no")
	}
	if steps.started != 0 || steps.windowStarted != 0 {
		t.Error("nothing should start after a no")
	}
	if !strings.Contains(out.String(), "No changes made") {
		t.Errorf("unclear output:\n%s", out)
	}
}

func TestWithoutATerminalItOnlyDescribesThePlan(t *testing.T) {
	original := "export A=1\n"
	rc := rcFile(t, original)
	e, steps, out := env(t, rc, "", false, false)
	if err := runInitWith(e); err != nil {
		t.Fatal(err)
	}
	if read(t, rc) != original || steps.started != 0 {
		t.Error("a non-interactive run without --yes must change nothing")
	}
	if !strings.Contains(out.String(), hookLine) || !strings.Contains(out.String(), "Nothing was changed") {
		t.Errorf("it should show the plan and say nothing changed:\n%s", out)
	}
}

func TestYesAcceptsTheDefaultsButNotAutoStartAtLogin(t *testing.T) {
	rc := rcFile(t, "")
	e, steps, _ := env(t, rc, "", false, true)
	if err := runInitWith(e); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, rc), hookLine) || steps.started != 1 || steps.windowStarted != 1 {
		t.Errorf("--yes should set up the hook, start Po and show the pet: %+v", steps)
	}
	if steps.loginTurnedOn != 0 {
		t.Error("--yes must not turn on starting at login; that is a bigger change to make unasked")
	}
}

func TestAnsweringYesToLoginTurnsItOn(t *testing.T) {
	rc := rcFile(t, "")
	e, steps, _ := env(t, rc, "y\ny\ny\n", true, false)
	runInitWith(e)
	if steps.loginTurnedOn != 1 {
		t.Errorf("login was turned on %d times, want 1", steps.loginTurnedOn)
	}
}

func TestWithoutTheWindowProgramItSaysHowToGetIt(t *testing.T) {
	rc := rcFile(t, "")
	e, steps, out := env(t, rc, "y\n", true, false)
	e.windowAvailable = func() bool { return false }
	runInitWith(e)
	if steps.windowStarted != 0 || !strings.Contains(out.String(), "desktop/install.sh") {
		t.Errorf("unhelpful output:\n%s", out)
	}
}

func TestThePathLineIsAddedOnlyWhenNeeded(t *testing.T) {
	for name, tc := range map[string]struct {
		onPath    bool
		existing  string
		wantAdded bool
	}{
		"not on the path":             {false, "", true},
		"already on the path":         {true, "", false},
		"the line is already in file": {false, "export PATH=\"$HOME/go/bin:$PATH\"\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			rc := rcFile(t, tc.existing)
			e, _, _ := env(t, rc, "y\nn\n", true, false)
			e.onPath = func() bool { return tc.onPath }
			runInitWith(e)
			got := read(t, rc)
			has := strings.Contains(got, pathMarker)
			if has != tc.wantAdded {
				t.Errorf("path block present = %v, want %v:\n%s", has, tc.wantAdded, got)
			}
			if tc.wantAdded {
				if strings.Index(got, "export PATH") > strings.Index(got, hookLine) {
					t.Error("the PATH line must come before the hook, or the hook cannot find po")
				}
				if !strings.Contains(got, `export PATH="$HOME/go/bin:$PATH"`) {
					t.Errorf("home folder not written as $HOME:\n%s", got)
				}
			}
		})
	}
}

func TestInitThenUndoRestoresTheOriginalExactly(t *testing.T) {
	for name, original := range map[string]string{
		"a normal file":        "export A=1\nalias ll='ls -l'\n",
		"no final newline":     "export A=1",
		"an empty file":        "",
		"lines after the spot": "# comment\n\nexport B=2\n\n\nexport C=3\n",
	} {
		t.Run(name, func(t *testing.T) {
			rc := rcFile(t, original)
			e, _, _ := env(t, rc, "y\nn\n", true, false)
			e.onPath = func() bool { return false }
			if err := runInitWith(e); err != nil {
				t.Fatal(err)
			}
			e2, steps, _ := env(t, rc, "", false, false)
			if err := undoInit(e2); err != nil {
				t.Fatal(err)
			}
			got := read(t, rc)
			// A file with no final newline gains one; otherwise it is byte for byte the same.
			want := original
			if want != "" && !strings.HasSuffix(want, "\n") {
				want += "\n"
			}
			if got != want {
				t.Errorf("undo did not restore the file.\nwant %q\n got %q", want, got)
			}
			if steps.stopped != 1 {
				t.Error("undo should stop Po and the window")
			}
		})
	}
}

func TestUndoLeavesAHookTheUserWroteThemselves(t *testing.T) {
	original := "export A=1\neval \"$(po hook zsh)\"\n"
	rc := rcFile(t, original)
	e, _, out := env(t, rc, "", false, false)
	if err := undoInit(e); err != nil {
		t.Fatal(err)
	}
	if read(t, rc) != original {
		t.Error("undo removed a line that init did not add")
	}
	if !strings.Contains(out.String(), "left it alone") && !strings.Contains(out.String(), "left alone") {
		t.Errorf("it should explain:\n%s", out)
	}
}

func TestUndoWhenNothingWasAdded(t *testing.T) {
	rc := rcFile(t, "export A=1\n")
	e, _, out := env(t, rc, "", false, false)
	undoInit(e)
	if !strings.Contains(out.String(), "Nothing to remove") || read(t, rc) != "export A=1\n" {
		t.Errorf("unexpected: %s", out)
	}
}

func TestTheBackupIsNeverOverwritten(t *testing.T) {
	rc := rcFile(t, "first\n")
	if _, err := backupOnce(rc, ".po-backup"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(rc, []byte("second\n"), 0o644)
	if _, err := backupOnce(rc, ".po-backup"); err != nil {
		t.Fatal(err)
	}
	if read(t, rc+".po-backup") != "first\n" {
		t.Error("a later backup replaced the original")
	}
}

func TestFilePermissionsArePreserved(t *testing.T) {
	rc := rcFile(t, "export SECRET=1\n")
	os.Chmod(rc, 0o600)
	e, _, _ := env(t, rc, "y\nn\n", true, false)
	runInitWith(e)
	for _, p := range []string{rc, rc + ".po-backup"} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s has mode %v, want 0600 (the file may hold secrets)", filepath.Base(p), info.Mode().Perm())
		}
	}
}

func TestDevNullIsNotATerminal(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("/dev/null cannot answer a question, so it is not a terminal")
	}
}

func TestInputThatEndsWithoutAnAnswerIsNotConsent(t *testing.T) {
	original := "export A=1\n"
	rc := rcFile(t, original)
	// The terminal check passes, but the input is empty, as if it were /dev/null.
	e, steps, _ := env(t, rc, "", true, false)
	if err := runInitWith(e); err != nil {
		t.Fatal(err)
	}
	if read(t, rc) != original {
		t.Error("the file was changed although nobody said yes")
	}
	if steps.started != 0 {
		t.Error("Po was started although nobody said yes")
	}
}

func TestEnterAloneTakesTheDefault(t *testing.T) {
	rc := rcFile(t, "")
	e, _, _ := env(t, rc, "\n\n", true, false) // Enter, Enter: accept, and decline the optional ones
	runInitWith(e)
	if !strings.Contains(read(t, rc), hookLine) {
		t.Error("pressing Enter should accept the default of yes for the main change")
	}
}
