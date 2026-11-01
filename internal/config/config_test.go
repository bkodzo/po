package config

import (
	"os"
	"path/filepath"
	"testing"
)

func use(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	t.Setenv("PO_CONFIG", path)
	return path
}

func TestMissingFileGivesDefaults(t *testing.T) {
	use(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c != Default() {
		t.Errorf("got %+v, want defaults", c)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	use(t)
	c := Default()
	for k, v := range map[string]string{
		"reader": "decision", "url": "https://api.typesafe.ai", "model": "jev-latest",
		"key_env": "TYPESAFE_API_KEY", "gate": "0.7", "idle_minutes": "3",
		"local_model": "/models/x.gguf",
	} {
		if err := c.Set(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != c {
		t.Errorf("got %+v, want %+v", got, c)
	}
}

func TestBadValuesAreRejected(t *testing.T) {
	c := Default()
	for k, v := range map[string]string{
		"reader": "magic", "gate": "2", "gate ": "x", "idle_minutes": "0", "nope": "1",
	} {
		if c.Set(k, v) == nil {
			t.Errorf("Set(%q, %q) should fail", k, v)
		}
	}
}

func TestLoadReportsBadLines(t *testing.T) {
	path := use(t)
	os.WriteFile(path, []byte("reader = rules\nthis is wrong\n"), 0o644)
	if _, err := Load(); err == nil {
		t.Error("expected an error for a line without =")
	}
}

func TestCommentsAndQuotesAreAllowed(t *testing.T) {
	path := use(t)
	os.WriteFile(path, []byte("# note\n\nreader = \"rules\"\n"), 0o644)
	c, err := Load()
	if err != nil || c.Reader != "rules" {
		t.Errorf("got %+v, %v", c, err)
	}
}

func TestReadKeyPrefersFile(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "k")
	os.WriteFile(keyFile, []byte("  from-file\n"), 0o600)
	t.Setenv("MY_KEY", "from-env")
	c := Default()
	c.KeyEnv, c.KeyFile = "MY_KEY", keyFile
	if got := c.ReadKey(); got != "from-file" {
		t.Errorf("got %q", got)
	}
	c.KeyFile = ""
	if got := c.ReadKey(); got != "from-env" {
		t.Errorf("got %q", got)
	}
}

func TestOverlaySetting(t *testing.T) {
	use(t)
	c := Default()
	if c.Overlay != "off" {
		t.Errorf("the corner pet should default to off (the desktop window is the main pet), got %q", c.Overlay)
	}
	if err := c.Set("overlay", "on"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil || got.Overlay != "on" {
		t.Errorf("did not round trip: %+v, %v", got, err)
	}
	if c.Set("overlay", "maybe") == nil {
		t.Error("only on and off should be accepted")
	}
}

func TestIdleSettings(t *testing.T) {
	use(t)
	c := Default()
	if c.Idle != "on" || c.SleepMinutes != 10 {
		t.Errorf("defaults wrong: idle %q, sleep %d", c.Idle, c.SleepMinutes)
	}
	if err := c.Set("idle", "off"); err != nil {
		t.Fatal(err)
	}
	if err := c.Set("sleep_minutes", "3"); err != nil {
		t.Fatal(err)
	}
	if c.Save() != nil {
		t.Fatal("save failed")
	}
	got, err := Load()
	if err != nil || got.Idle != "off" || got.SleepMinutes != 3 {
		t.Errorf("did not round trip: %+v, %v", got, err)
	}
	for k, v := range map[string]string{"idle": "sometimes", "sleep_minutes": "0"} {
		if c.Set(k, v) == nil {
			t.Errorf("%s = %s should be rejected", k, v)
		}
	}
}
