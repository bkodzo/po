package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Config is Po's settings. The file is plain "key = value" lines, so it can be
// edited by hand or with `po config set`.
type Config struct {
	// Reader is auto, rules, keyword, decision or ollama. Auto answers with the
	// rules when they are sure and asks the model only when they are not.
	Reader string
	// URL and Model point at a decision server or an Ollama server. They can be
	// local (a model Po manages) or hosted (your own Jev, Clef or similar API).
	URL   string
	Model string
	// KeyEnv names an environment variable holding an API key; KeyFile names a
	// file holding one. The key itself is never stored in this file.
	KeyEnv  string
	KeyFile string
	// LocalModel is a model file for the local server Po starts on demand.
	LocalModel string
	// Gate is how sure the fast layer must be to skip the model.
	Gate float64
	// IdleMinutes is how long Po's own model server may sit unused before it
	// is shut down to free memory.
	IdleMinutes int
	Theme       string
	// Overlay is "on" to draw the small pet in the corner of each terminal, or
	// "off" (the default) to leave the terminal text alone. The corner pet is
	// made of text characters, so selecting text near it copies the pet too; the
	// desktop window (po window) has no such problem.
	Overlay string
	// Idle is "on" for little animations while nothing is happening (looking
	// around, stretching, sleeping), or "off" for a pet that only reacts.
	Idle string
	// SleepMinutes is how long with no commands before the pet falls asleep.
	SleepMinutes int
}

func Default() Config {
	return Config{
		Reader:       "auto",
		URL:          "http://127.0.0.1:8080",
		Model:        "clef-flash",
		KeyEnv:       "PO_API_KEY",
		Gate:         0.6,
		IdleMinutes:  10,
		Theme:        "soccer",
		Overlay:      "off",
		Idle:         "on",
		SleepMinutes: 10,
	}
}

// Path is the config file. PO_CONFIG overrides it.
func Path() string {
	if p := os.Getenv("PO_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "po", "config")
}

// Load reads the config, filling anything missing with defaults. A missing file
// is not an error.
func Load() (Config, error) {
	c := Default()
	path := Path()
	if path == "" {
		return c, nil
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return c, fmt.Errorf("%s:%d: expected key = value", path, n)
		}
		if err := c.Set(strings.TrimSpace(key), strings.Trim(strings.TrimSpace(value), `"`)); err != nil {
			return c, fmt.Errorf("%s:%d: %w", path, n, err)
		}
	}
	return c, scanner.Err()
}

var readers = map[string]bool{"auto": true, "rules": true, "keyword": true, "decision": true, "ollama": true}

// Keys lists every setting name.
func Keys() []string {
	k := []string{"reader", "url", "model", "key_env", "key_file", "local_model", "gate", "idle_minutes", "theme", "overlay", "idle", "sleep_minutes"}
	sort.Strings(k)
	return k
}

func (c *Config) Set(key, value string) error {
	switch key {
	case "reader":
		if !readers[value] {
			return fmt.Errorf("reader must be auto, rules, keyword, decision or ollama, not %q", value)
		}
		c.Reader = value
	case "url":
		c.URL = value
	case "model":
		c.Model = value
	case "key_env":
		c.KeyEnv = value
	case "key_file":
		c.KeyFile = value
	case "local_model":
		c.LocalModel = value
	case "gate":
		g, err := strconv.ParseFloat(value, 64)
		if err != nil || g < 0 || g > 1 {
			return fmt.Errorf("gate must be a number from 0 to 1")
		}
		c.Gate = g
	case "idle_minutes":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("idle_minutes must be a whole number of at least 1")
		}
		c.IdleMinutes = n
	case "theme":
		c.Theme = value
	case "idle":
		if value != "on" && value != "off" {
			return fmt.Errorf("idle must be on or off, not %q", value)
		}
		c.Idle = value
	case "sleep_minutes":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("sleep_minutes must be a whole number of at least 1")
		}
		c.SleepMinutes = n
	case "overlay":
		if value != "on" && value != "off" {
			return fmt.Errorf("overlay must be on or off, not %q", value)
		}
		c.Overlay = value
	default:
		return fmt.Errorf("unknown setting %q (known: %s)", key, strings.Join(Keys(), ", "))
	}
	return nil
}

func (c Config) Get(key string) (string, error) {
	switch key {
	case "reader":
		return c.Reader, nil
	case "url":
		return c.URL, nil
	case "model":
		return c.Model, nil
	case "key_env":
		return c.KeyEnv, nil
	case "key_file":
		return c.KeyFile, nil
	case "local_model":
		return c.LocalModel, nil
	case "gate":
		return strconv.FormatFloat(c.Gate, 'f', -1, 64), nil
	case "idle_minutes":
		return strconv.Itoa(c.IdleMinutes), nil
	case "theme":
		return c.Theme, nil
	case "overlay":
		return c.Overlay, nil
	case "idle":
		return c.Idle, nil
	case "sleep_minutes":
		return strconv.Itoa(c.SleepMinutes), nil
	}
	return "", fmt.Errorf("unknown setting %q (known: %s)", key, strings.Join(Keys(), ", "))
}

// Save writes every setting, so the file doubles as documentation of what is in
// effect.
func (c Config) Save() error {
	path := Path()
	if path == "" {
		return fmt.Errorf("no config directory")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Po settings. Edit by hand or use `po config set key value`.\n")
	for _, k := range Keys() {
		v, _ := c.Get(k)
		fmt.Fprintf(&b, "%s = %s\n", k, v)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// ReadKey returns the API key from the file or the environment variable, or an
// empty string if neither is set.
func (c Config) ReadKey() string {
	if c.KeyFile != "" {
		if data, err := os.ReadFile(c.KeyFile); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	if c.KeyEnv != "" {
		return os.Getenv(c.KeyEnv)
	}
	return ""
}
