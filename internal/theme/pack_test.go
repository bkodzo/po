package theme

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bkodzo/po/internal/mood"
)

func themesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PO_THEMES_DIR", dir)
	return dir
}

// copyPack gives a fresh deep copy of a pack so a test can break it.
func copyPack(t *testing.T, p Pack) *Pack {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	q, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func hasIssue(issues []string, want string) bool {
	for _, i := range issues {
		if strings.Contains(i, want) {
			return true
		}
	}
	return false
}

// The core promise: a theme written out and loaded back looks exactly the same.
func TestAnExportedThemeLoadsBackAndDrawsIdentically(t *testing.T) {
	themesDir(t)
	for _, name := range []string{"soccer", "ghost", "cat"} {
		t.Run(name, func(t *testing.T) {
			orig, _ := Get(name)
			data, err := json.Marshal(orig.Export())
			if err != nil {
				t.Fatal(err)
			}
			p, issues, err := ParseAndValidate(data)
			if err != nil || len(issues) > 0 {
				t.Fatalf("an exported theme must validate: %v %v", err, issues)
			}
			loaded, err := FromPack(p)
			if err != nil {
				t.Fatal(err)
			}
			same := func(what string, a, b interface{ Letters() string }) {
				if a.Letters() != b.Letters() {
					t.Fatalf("%s differs after the round trip", what)
				}
			}
			for _, m := range mood.All {
				for tick := 0; tick < 200; tick++ {
					a, pa := orig.Frame(m, tick)
					b, pb := loaded.Frame(m, tick)
					same(string(m)+" full", a, b)
					for k, c := range pa {
						if pb[k] != c {
							t.Fatalf("%s palette letter %q differs", m, k)
						}
					}
					ma, _ := orig.MiniFrame(m, tick)
					mb, _ := loaded.MiniFrame(m, tick)
					same(string(m)+" mini", ma, mb)
				}
			}
			for _, act := range ActivityNames() {
				info, _ := ActivityOf(act)
				li, _ := loaded.ActivityInfo(act)
				if li != info {
					t.Fatalf("%s info %+v, want %+v", act, li, info)
				}
				for tick := 0; tick < info.Ticks+20; tick++ {
					a, _ := orig.ActivityFrame(act, tick)
					b, _ := loaded.ActivityFrame(act, tick)
					same(act, a, b)
				}
			}
			if loaded.Name != name || len(loaded.Lines[mood.Frustrated]) == 0 || len(loaded.Milestones["relief"]) == 0 || len(loaded.Recaps["rough"]) == 0 {
				t.Errorf("the words did not survive: %+v", loaded.Name)
			}
			// And exporting the loaded theme gives the same file again.
			again, _ := json.Marshal(loaded.Export())
			if string(again) != string(data) {
				t.Error("exporting the loaded theme does not reproduce the original file")
			}
		})
	}
}

func TestAReskinKeepsTheShapesButChangesColoursAndWords(t *testing.T) {
	themesDir(t)
	p, issues, err := ParseAndValidate([]byte(`{
		"name": "mango", "base": "soccer",
		"palette": {"b": [250, 200, 20]},
		"tints": {"frustrated": {"color": [0, 0, 255], "strength": 0.5}},
		"lines": {"frustrated": ["Mango is not amused."]},
		"milestones": {"relief": ["Sweet relief after %d failures."]}
	}`))
	if err != nil || len(issues) > 0 {
		t.Fatalf("%v %v", err, issues)
	}
	mango, err := FromPack(p)
	if err != nil {
		t.Fatal(err)
	}
	if mango.Name != "mango" {
		t.Errorf("name %q", mango.Name)
	}
	// The drawing is the base's drawing.
	for _, m := range mood.All {
		a, _ := Soccer.Frame(m, 11)
		b, _ := mango.Frame(m, 11)
		if a.Letters() != b.Letters() {
			t.Errorf("%s: a reskin must not change the shapes", m)
		}
	}
	_, calm := mango.Frame(mood.Calm, 0)
	if calm['b'].R != 250 || calm['b'].G != 200 {
		t.Errorf("body colour not changed: %+v", calm['b'])
	}
	_, base := Soccer.Frame(mood.Calm, 0)
	if calm['o'] != base['o'] {
		t.Error("letters the file did not mention must keep the base colour")
	}
	_, angry := mango.Frame(mood.Frustrated, 0)
	if angry['b'].B <= calm['b'].B {
		t.Errorf("the blue tint for frustrated was not applied: %+v vs %+v", angry['b'], calm['b'])
	}
	if got := mango.Say(mood.Frustrated); got != "Mango is not amused." {
		t.Errorf("frustrated line %q", got)
	}
	if len(mango.Lines[mood.Tired]) == 0 || mango.Lines[mood.Tired][0] != Soccer.Lines[mood.Tired][0] {
		t.Error("moods the file did not mention keep the base's lines")
	}
	if got := mango.MilestoneLine("relief", 4); got != "Sweet relief after 4 failures." {
		t.Errorf("relief line %q", got)
	}
	if len(mango.Milestones["record"]) == 0 {
		t.Error("kinds the file did not mention keep the base's lines")
	}
}

func TestValidationReportsEveryProblemAtOnce(t *testing.T) {
	good := Soccer.Export()
	p := copyPack(t, good)
	p.Name = "Bad Name!"
	m := p.Moods["calm"]
	m.Full.Frames[2] = m.Full.Frames[2][:10] // too short
	m.Full.Sequence[0] = 99                  // out of range
	m.Mini.W = 7                             // wrong mini size
	p.Moods["calm"] = m
	delete(p.Moods, "tired")                            // missing mood
	p.Lines["calm"] = []string{strings.Repeat("x", 90)} // too long
	p.Milestones = map[string][]string{"relief": {"%d and %d"}, "bogus": {"hi"}}
	p.Fps = map[string]int{"full": 99}
	issues := p.Validate()
	for _, want := range []string{
		"name: must be lowercase", "moods.calm.full.frames[2]: 10 letters, want 480",
		"moods.calm.full.sequence[0]: points at frame 99", "moods.calm.mini: must be 8x4",
		"the tired mood is missing", "too long for the speech bubble", `only one "%d"`,
		`unknown kind "bogus"`, "fps: only full and mini",
	} {
		if !hasIssue(issues, want) {
			t.Errorf("no issue mentioning %q in:\n  %s", want, strings.Join(issues, "\n  "))
		}
	}
	if len(issues) < 8 {
		t.Errorf("expected all the problems together, got %d:\n  %s", len(issues), strings.Join(issues, "\n  "))
	}
}

func TestValidationCatchesTheMistakesAuthorsMake(t *testing.T) {
	cases := map[string]struct {
		json string
		want string
	}{
		"no base and no moods":     {`{"name":"x"}`, "a reskin needs a base"},
		"unknown base":             {`{"name":"x","base":"dragon"}`, `no built-in character called "dragon"`},
		"both base and moods":      {`{"name":"x","base":"soccer","moods":{"calm":{}}}`, "use either base"},
		"a palette key too long":   {`{"name":"x","base":"soccer","palette":{"bb":[1,2,3]}}`, "single letters"},
		"a colour out of range":    {`{"name":"x","base":"soccer","palette":{"b":[1,2,300]}}`, "0 to 255"},
		"transparent can't colour": {`{"name":"x","base":"soccer","palette":{".":[1,2,3]}}`, "transparent"},
		"an unknown tint mood":     {`{"name":"x","base":"soccer","tints":{"calm":{"color":[1,2,3],"strength":0.5}}}`, "unknown mood"},
		"a tint too strong":        {`{"name":"x","base":"soccer","tints":{"tired":{"color":[1,2,3],"strength":2}}}`, "from 0 to 1"},
		"an empty line":            {`{"name":"x","base":"soccer","lines":{"calm":["  "]}}`, "cannot be empty"},
		"an unknown recap tone":    {`{"name":"x","base":"soccer","recaps":{"epic":["hi"]}}`, `unknown tone "epic"`},
		"a wrong version":          {`{"name":"x","version":7,"base":"soccer"}`, "version"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := Parse([]byte(tc.json))
			if err != nil {
				t.Fatal(err)
			}
			if issues := p.Validate(); !hasIssue(issues, tc.want) {
				t.Errorf("want an issue mentioning %q, got %v", tc.want, issues)
			}
		})
	}
}

func TestMisspeltFieldsAreRejectedNotIgnored(t *testing.T) {
	if _, err := Parse([]byte(`{"name":"x","base":"soccer","pallete":{}}`)); err == nil || !strings.Contains(err.Error(), "pallete") {
		t.Errorf("a typo should be an error naming it, got %v", err)
	}
	if _, err := Parse([]byte(`not json`)); err == nil {
		t.Error("not json should be an error")
	}
}

func TestFramesCanBeWrittenAsRowsOrOneString(t *testing.T) {
	var a, b struct{ F FrameText }
	if err := json.Unmarshal([]byte(`{"F":"ab.cd.ef."}`), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"F":["ab.","cd.","ef."]}`), &b); err != nil {
		t.Fatal(err)
	}
	if a.F != b.F || a.F != "ab.cd.ef." {
		t.Errorf("%q vs %q", a.F, b.F)
	}
	if err := json.Unmarshal([]byte(`{"F":5}`), &a); err == nil {
		t.Error("a number is not a frame")
	}
}

func TestAHandDrawnFullPackLoadsAndDraws(t *testing.T) {
	// The smallest legal pack: every mood has the same 8x8 square as its picture.
	rows := `["bbbbbbbb","bkbbbbkb","bbbbbbbb","bbbbbbbb","bbmmmmbb","bbbbbbbb","bbbbbbbb","bbbbbbbb"]`
	mini := `["bbbbbbbb","bkbbbbkb","bbmmmmbb","bbbbbbbb"]`
	mood := func() string {
		return `{"palette":{"b":[200,100,50],"k":[0,0,0],"m":[150,0,0]},
			"full":{"w":8,"h":8,"frames":[` + rows + `],"sequence":[0]},
			"mini":{"w":8,"h":4,"frames":[` + mini + `],"sequence":[0]}}`
	}
	text := `{"version":1,"name":"square","moods":{"calm":` + mood() + `,"frustrated":` + mood() + `,"celebrating":` + mood() +
		`,"tired":` + mood() + `,"focused":` + mood() + `},
		"lines":{"calm":["Square."],"frustrated":["Square."],"celebrating":["Square."],"tired":["Square."],"focused":["Square."]},
		"unsure":["Square?"]}`
	p, issues, err := ParseAndValidate([]byte(text))
	if err != nil || len(issues) > 0 {
		t.Fatalf("%v %v", err, issues)
	}
	th, err := FromPack(p)
	if err != nil {
		t.Fatal(err)
	}
	c, pal := th.Frame(mood2("calm"), 3)
	if c.W != 8 || c.H != 8 || pal['b'].R != 200 || !strings.Contains(c.Letters(), "mmmm") {
		t.Errorf("the hand-drawn picture did not come through: %dx%d\n%s", c.W, c.H, c.Letters())
	}
	if got := th.Say(mood2("tired")); got != "Square." {
		t.Errorf("line %q", got)
	}
	// It has no activities of its own, so asking for one falls back to the calm face.
	if a, _ := th.ActivityFrame("sleep", 5); a.W != 8 {
		t.Error("a missing activity should fall back to the calm picture")
	}
	if exp := th.Export(); exp.Name != "square" || len(exp.Available) < 3 {
		t.Errorf("export of a loaded theme: %q %v", exp.Name, exp.Available)
	}
}

func mood2(s string) mood.Mood { return mood.Mood(s) }

func TestInstallListUseAndRemove(t *testing.T) {
	dir := themesDir(t)
	src := filepath.Join(t.TempDir(), "mango.json")
	os.WriteFile(src, []byte(`{"name":"mango","base":"cat","lines":{"calm":["Mango."]}}`), 0o644)

	name, err := Install(src)
	if err != nil || name != "mango" {
		t.Fatalf("install: %q %v", name, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "mango.json")); err != nil {
		t.Error("the file was not copied in")
	}
	got, ok := Get("mango")
	if !ok || got.Say(mood.Calm) != "Mango." {
		t.Errorf("an installed theme should be usable: %v %q", ok, got.Say(mood.Calm))
	}
	listed := false
	for _, n := range Names() {
		listed = listed || n == "mango"
	}
	if !listed {
		t.Errorf("Names() = %v should include mango", Names())
	}
	if err := Remove("mango"); err != nil {
		t.Fatal(err)
	}
	if _, ok := Get("mango"); ok {
		t.Error("a removed theme must be gone")
	}
	if Remove("mango") == nil {
		t.Error("removing twice should say there is no such theme")
	}
}

func TestInstallRefusesBadFilesAndBuiltInNames(t *testing.T) {
	dir := themesDir(t)
	write := func(text string) string {
		p := filepath.Join(t.TempDir(), "t.json")
		os.WriteFile(p, []byte(text), 0o644)
		return p
	}
	if _, err := Install(write(`{"name":"soccer","base":"cat"}`)); err == nil || !strings.Contains(err.Error(), "built-in") {
		t.Errorf("a built-in name must be refused: %v", err)
	}
	if _, err := Install(write(`{"name":"x","base":"nope"}`)); err == nil || !strings.Contains(err.Error(), "problem") {
		t.Errorf("an invalid theme must be refused with its problems: %v", err)
	}
	if _, err := Install(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("a missing file is an error")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("nothing should have been installed, found %d file(s)", len(entries))
	}
	if Remove("soccer") == nil || Remove("ghost") == nil {
		t.Error("built-in characters cannot be removed")
	}
}

func TestANameCannotEscapeTheThemesFolder(t *testing.T) {
	dir := themesDir(t)
	outside := filepath.Join(filepath.Dir(dir), "evil.json")
	os.WriteFile(outside, []byte(`{"name":"evil","base":"soccer"}`), 0o644)
	defer os.Remove(outside)
	for _, name := range []string{"../evil", "..", "a/b", "/etc/passwd", "", "Evil", "x y"} {
		if _, ok := Get(name); ok {
			t.Errorf("Get(%q) found something", name)
		}
		if Remove(name) == nil {
			t.Errorf("Remove(%q) should refuse", name)
		}
	}
}

func TestABrokenFileOnDiskIsNotOffered(t *testing.T) {
	dir := themesDir(t)
	os.WriteFile(filepath.Join(dir, "broken.json"), []byte(`{"name":"broken","base":"nope"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o644)
	for _, n := range Names() {
		if n == "broken" {
			t.Error("a theme that does not load must not be listed")
		}
	}
	if _, ok := Get("broken"); ok {
		t.Error("a theme that does not load must not be usable")
	}
}

func TestAHugeFileIsRefused(t *testing.T) {
	big := `{"name":"x","base":"soccer","lines":{"calm":["` + strings.Repeat("a", maxThemeFile) + `"]}}`
	if _, _, err := ParseAndValidate([]byte(big)); err == nil || !strings.Contains(err.Error(), "KB") {
		t.Errorf("an oversize file should be refused: %v", err)
	}
}

func TestScaffoldIsAValidStartingPoint(t *testing.T) {
	themesDir(t)
	data, err := Scaffold("mango", "ghost")
	if err != nil {
		t.Fatal(err)
	}
	p, issues, err := ParseAndValidate(data)
	if err != nil || len(issues) > 0 {
		t.Fatalf("the scaffold must validate as it is: %v %v", err, issues)
	}
	th, _ := FromPack(p)
	for _, m := range mood.All {
		a, pa := Ghost.Frame(m, 5)
		b, pb := th.Frame(m, 5)
		if a.Letters() != b.Letters() || pa['b'] != pb['b'] {
			t.Errorf("%s: a scaffold must start identical to its base", m)
		}
	}
	if _, err := Scaffold("soccer", "cat"); err == nil {
		t.Error("a built-in name cannot be used")
	}
	if _, err := Scaffold("Bad Name", "cat"); err == nil {
		t.Error("an invalid name cannot be used")
	}
	if _, err := Scaffold("ok", "dragon"); err == nil {
		t.Error("an unknown base cannot be used")
	}
}
