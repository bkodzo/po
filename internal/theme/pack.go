package theme

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bkodzo/po/internal/mood"
	"github.com/bkodzo/po/internal/sprite"
)

// FrameText is one picture, as a letter for every pixel in row order. In a theme
// file it can be written as one string, or as a list of rows, which is much
// easier to draw by hand.
type FrameText string

func (f *FrameText) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*f = FrameText(s)
		return nil
	}
	var rows []string
	if err := json.Unmarshal(b, &rows); err != nil {
		return errors.New("a frame must be a string of letters or a list of rows")
	}
	*f = FrameText(strings.Join(rows, ""))
	return nil
}

const maxThemeFile = 2 << 20 // 2 MB

var nameRule = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

var tintedMoods = []string{"frustrated", "celebrating", "tired", "focused"}

func builtin(name string) (Theme, bool) {
	t, ok := themes[name]
	return t, ok
}

// UserDir is where installed themes live. PO_THEMES_DIR overrides it.
func UserDir() string {
	if d := os.Getenv("PO_THEMES_DIR"); d != "" {
		return d
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "po", "themes")
}

// ---- Validation ----------------------------------------------------------

type checker struct{ issues []string }

func (c *checker) add(where, format string, args ...any) {
	c.issues = append(c.issues, where+": "+fmt.Sprintf(format, args...))
}

func (c *checker) colour(where string, rgb [3]int) {
	for _, v := range rgb {
		if v < 0 || v > 255 {
			c.add(where, "colour values must be from 0 to 255, got %v", rgb)
			return
		}
	}
}

func (c *checker) palette(where string, p map[string][3]int) {
	for letter, rgb := range p {
		if len([]rune(letter)) != 1 {
			c.add(where, "palette keys are single letters, got %q", letter)
			continue
		}
		if letter == "." {
			c.add(where, `"." means transparent and cannot have a colour`)
			continue
		}
		c.colour(fmt.Sprintf("%s[%q]", where, letter), rgb)
	}
}

func (c *checker) animation(where string, a Animation, pal map[string][3]int, wantW, wantH int, minSize, maxSize int) {
	if a.W < 1 || a.H < 1 {
		c.add(where, "w and h must be positive, got %dx%d", a.W, a.H)
		return
	}
	if wantW > 0 && (a.W != wantW || a.H != wantH) {
		c.add(where, "must be %dx%d, got %dx%d", wantW, wantH, a.W, a.H)
	}
	if wantW == 0 && (a.W < minSize || a.W > maxSize || a.H < minSize || a.H > maxSize) {
		c.add(where, "size must be between %d and %d pixels each way, got %dx%d", minSize, maxSize, a.W, a.H)
	}
	if len(a.Frames) == 0 {
		c.add(where+".frames", "at least one frame is needed")
	}
	if len(a.Sequence) == 0 {
		c.add(where+".sequence", "the sequence says which frame to show at each tick, and it is empty")
	}
	missing := map[rune]bool{}
	for i, f := range a.Frames {
		if len(f) != a.W*a.H {
			c.add(fmt.Sprintf("%s.frames[%d]", where, i), "%d letters, want %d (%d wide and %d tall)", len(f), a.W*a.H, a.W, a.H)
			continue
		}
		for _, r := range string(f) {
			if r == '.' {
				continue
			}
			if _, ok := pal[string(r)]; !ok {
				missing[r] = true
			}
		}
	}
	if len(missing) > 0 {
		var letters []string
		for r := range missing {
			letters = append(letters, string(r))
		}
		sort.Strings(letters)
		c.add(where, "draws letters with no colour in the palette: %s", strings.Join(letters, " "))
	}
	for i, idx := range a.Sequence {
		if idx < 0 || idx >= len(a.Frames) {
			c.add(fmt.Sprintf("%s.sequence[%d]", where, i), "points at frame %d, but there are %d frames", idx, len(a.Frames))
			break // one is enough to show the problem
		}
	}
}

func (c *checker) words(where string, m map[string][]string, known []string, what string) {
	allowed := map[string]bool{}
	for _, k := range known {
		allowed[k] = true
	}
	for key, lines := range m {
		if !allowed[key] {
			c.add(where, "unknown %s %q (known: %s)", what, key, strings.Join(known, ", "))
			continue
		}
		if len(lines) == 0 {
			c.add(where+"."+key, "needs at least one line")
		}
		for i, line := range lines {
			w := fmt.Sprintf("%s.%s[%d]", where, key, i)
			if strings.TrimSpace(line) == "" {
				c.add(w, "a line cannot be empty")
			}
			if len([]rune(line)) > 80 {
				c.add(w, "%d characters is too long for the speech bubble (80 at most)", len([]rune(line)))
			}
			if strings.Count(line, "%d") > 1 || strings.Contains(strings.ReplaceAll(line, "%d", ""), "%") {
				c.add(w, `only one "%%d" is allowed, and no other "%%"`)
			}
			if strings.ContainsAny(line, "\n\r\t") {
				c.add(w, "a line cannot contain line breaks or tabs")
			}
		}
	}
}

// Validate checks a theme and returns every problem it finds, so an author can
// fix them all in one go.
func (p *Pack) Validate() []string {
	c := &checker{}
	if p.Version != 0 && p.Version != PackVersion {
		c.add("version", "this Po understands version %d, the file says %d", PackVersion, p.Version)
	}
	if !nameRule.MatchString(p.Name) {
		c.add("name", "must be lowercase letters, digits and dashes, starting with a letter (32 at most), got %q", p.Name)
	}
	if p.Fps != nil {
		for k, v := range p.Fps {
			if (k != "full" && k != "mini") || v < 1 || v > 30 {
				c.add("fps", "only full and mini are allowed, each from 1 to 30, got %s=%d", k, v)
			}
		}
	}

	explicit := len(p.Moods) > 0
	switch {
	case explicit && p.Base != "":
		c.add("base", "use either base (a reskin) or moods (a full pack), not both")
	case !explicit && p.Base == "":
		c.add("base", `a reskin needs a base such as "soccer", or the file needs a moods section`)
	case !explicit:
		if _, ok := builtin(p.Base); !ok {
			c.add("base", "no built-in character called %q (built in: %s)", p.Base, strings.Join(builtinNames(), ", "))
		}
	}

	if explicit {
		p.validateExplicit(c)
	} else {
		p.validateReskin(c)
	}

	if len(p.Lines) > 0 {
		moods := make([]string, 0, len(mood.All))
		for _, m := range mood.All {
			moods = append(moods, string(m))
		}
		c.words("lines", p.Lines, moods, "mood")
	}
	c.words("milestones", p.Milestones, MilestoneKinds, "kind")
	c.words("recaps", p.Recaps, RecapTones, "tone")
	if len(p.Unsure) > 0 {
		c.words("unsure", map[string][]string{"lines": p.Unsure}, []string{"lines"}, "key")
	}
	sort.Strings(c.issues)
	return c.issues
}

func (p *Pack) validateReskin(c *checker) {
	c.palette("palette", p.Palette)
	for m, t := range p.Tints {
		ok := false
		for _, k := range tintedMoods {
			ok = ok || k == m
		}
		if !ok {
			c.add("tints", "unknown mood %q (tints apply to: %s)", m, strings.Join(tintedMoods, ", "))
			continue
		}
		c.colour("tints."+m+".color", t.Color)
		if t.Strength < 0 || t.Strength > 1 {
			c.add("tints."+m+".strength", "must be from 0 to 1, got %v", t.Strength)
		}
	}
}

func (p *Pack) validateExplicit(c *checker) {
	if len(p.Palette) > 0 || len(p.Tints) > 0 {
		c.add("palette", "palette and tints belong to a reskin; a full pack gives each mood its own palette")
	}
	for _, m := range mood.All {
		mp, ok := p.Moods[string(m)]
		if !ok {
			c.add("moods", "the %s mood is missing", m)
			continue
		}
		where := "moods." + string(m)
		if len(mp.Palette) == 0 {
			c.add(where+".palette", "needs at least one colour")
		}
		c.palette(where+".palette", mp.Palette)
		c.animation(where+".full", mp.Full, mp.Palette, 0, 0, 8, 64)
		c.animation(where+".mini", mp.Mini, mp.Palette, MiniW, 4, 0, 0)
	}
	for m := range p.Moods {
		known := false
		for _, k := range mood.All {
			known = known || string(k) == m
		}
		if !known {
			c.add("moods", "unknown mood %q", m)
		}
	}
	if calm, ok := p.Moods["calm"]; ok {
		for name, a := range p.Activities {
			if _, known := activities[name]; !known {
				c.add("activities", "unknown animation %q (known: %s)", name, strings.Join(ActivityNames(), ", "))
				continue
			}
			c.palette("activities."+name+".palette", a.Palette)
			c.animation("activities."+name, a.Animation, a.Palette, calm.Full.W, calm.Full.H, 0, 0)
		}
	}
	for _, m := range mood.All {
		if len(p.Lines[string(m)]) == 0 {
			c.add("lines."+string(m), "a full pack needs at least one line for every mood")
		}
	}
	if len(p.Unsure) == 0 {
		c.add("unsure", "a full pack needs at least one line for when the pet is unsure")
	}
}

func builtinNames() []string {
	var n []string
	for k := range themes {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}

// ---- Loading -------------------------------------------------------------

// Parse reads a theme file strictly: a misspelled field is an error, not
// something quietly ignored.
func Parse(data []byte) (*Pack, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var p Pack
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("not a valid theme file: %w", err)
	}
	return &p, nil
}

// ParseAndValidate parses a theme file and checks it, returning the pack or all
// the problems found.
func ParseAndValidate(data []byte) (*Pack, []string, error) {
	if len(data) > maxThemeFile {
		return nil, nil, fmt.Errorf("the file is %d KB; theme files can be at most %d KB", len(data)/1024, maxThemeFile/1024)
	}
	p, err := Parse(data)
	if err != nil {
		return nil, nil, err
	}
	return p, p.Validate(), nil
}

func pickColour(rgb [3]int) sprite.Color {
	return sprite.RGB(uint8(rgb[0]), uint8(rgb[1]), uint8(rgb[2]))
}

// FromPack builds a theme from a validated pack.
func FromPack(p *Pack) (Theme, error) {
	if len(p.Moods) > 0 {
		cp := *p
		return Theme{
			Name: p.Name, Lines: asMoodLines(p.Lines), Unsure: p.Unsure,
			Milestones: p.Milestones, Recaps: p.Recaps, data: &cp,
		}, nil
	}
	base, ok := builtin(p.Base)
	if !ok {
		return Theme{}, fmt.Errorf("no built-in character called %q", p.Base)
	}
	t := base
	t.Name = p.Name

	t.Palette = sprite.Palette{}
	for k, v := range base.Palette {
		t.Palette[k] = v
	}
	for letter, rgb := range p.Palette {
		t.Palette[[]rune(letter)[0]] = pickColour(rgb)
	}
	t.Tints = map[mood.Mood]Tint{}
	for m, v := range base.Tints {
		t.Tints[m] = v
	}
	for m, v := range p.Tints {
		t.Tints[mood.Mood(m)] = Tint{Color: pickColour(v.Color), Strength: v.Strength}
	}
	t.Lines = map[mood.Mood][]string{}
	for m, v := range base.Lines {
		t.Lines[m] = v
	}
	for m, v := range p.Lines {
		if len(v) > 0 {
			t.Lines[mood.Mood(m)] = v
		}
	}
	if len(p.Unsure) > 0 {
		t.Unsure = p.Unsure
	}
	t.Milestones = mergeWords(base.Milestones, p.Milestones)
	t.Recaps = mergeWords(base.Recaps, p.Recaps)
	return t, nil
}

func asMoodLines(m map[string][]string) map[mood.Mood][]string {
	out := map[mood.Mood][]string{}
	for k, v := range m {
		out[mood.Mood(k)] = v
	}
	return out
}

func mergeWords(base, over map[string][]string) map[string][]string {
	out := map[string][]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		if len(v) > 0 {
			out[k] = v
		}
	}
	return out
}

// LoadFile reads, validates and builds a theme from a file.
func LoadFile(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, err
	}
	p, issues, err := ParseAndValidate(data)
	if err != nil {
		return Theme{}, err
	}
	if len(issues) > 0 {
		return Theme{}, fmt.Errorf("%d problem(s) in %s:\n  %s", len(issues), path, strings.Join(issues, "\n  "))
	}
	return FromPack(p)
}

// ---- Installed themes ----------------------------------------------------

func userPath(name string) (string, bool) {
	dir := UserDir()
	if dir == "" || !nameRule.MatchString(name) { // the rule also keeps a name from escaping the folder
		return "", false
	}
	return filepath.Join(dir, name+".json"), true
}

// Install checks a theme file and copies it into the installed themes.
func Install(src string) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	p, issues, err := ParseAndValidate(data)
	if err != nil {
		return "", err
	}
	if len(issues) > 0 {
		return "", fmt.Errorf("%d problem(s):\n  %s", len(issues), strings.Join(issues, "\n  "))
	}
	if _, isBuiltin := builtin(p.Name); isBuiltin {
		return "", fmt.Errorf("%q is a built-in character, so a theme cannot take its name", p.Name)
	}
	dest, ok := userPath(p.Name)
	if !ok {
		return "", errors.New("there is nowhere to keep installed themes")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", err
	}
	return p.Name, nil
}

// Remove deletes an installed theme. Built-in characters cannot be removed.
func Remove(name string) error {
	if _, isBuiltin := builtin(name); isBuiltin {
		return fmt.Errorf("%q is built in and cannot be removed", name)
	}
	path, ok := userPath(name)
	if !ok {
		return fmt.Errorf("%q is not a valid theme name", name)
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no installed theme called %q", name)
		}
		return err
	}
	return nil
}

// Installed lists the installed themes that load correctly.
func Installed() []string {
	dir := UserDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || !nameRule.MatchString(name) {
			continue
		}
		if _, isBuiltin := builtin(name); isBuiltin {
			continue
		}
		if _, err := LoadFile(filepath.Join(dir, e.Name())); err == nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// Scaffold writes a starting reskin for a built-in character, ready to edit.
func Scaffold(name, base string) ([]byte, error) {
	if !nameRule.MatchString(name) {
		return nil, fmt.Errorf("%q is not a valid theme name (lowercase letters, digits and dashes)", name)
	}
	if _, isBuiltin := builtin(name); isBuiltin {
		return nil, fmt.Errorf("%q is a built-in character, so a theme cannot take its name", name)
	}
	b, ok := builtin(base)
	if !ok {
		return nil, fmt.Errorf("no built-in character called %q (built in: %s)", base, strings.Join(builtinNames(), ", "))
	}
	p := Pack{Version: PackVersion, Name: name, Base: base, Palette: map[string][3]int{}, Tints: map[string]TintSpec{},
		Lines: map[string][]string{}, Unsure: b.Unsure, Milestones: b.Milestones, Recaps: b.Recaps}
	for letter, c := range b.Palette {
		p.Palette[string(letter)] = [3]int{int(c.R), int(c.G), int(c.B)}
	}
	for m, t := range b.Tints {
		p.Tints[string(m)] = TintSpec{Color: [3]int{int(t.Color.R), int(t.Color.G), int(t.Color.B)}, Strength: t.Strength}
	}
	for m, lines := range b.Lines {
		p.Lines[string(m)] = lines
	}
	return json.MarshalIndent(p, "", "  ")
}
