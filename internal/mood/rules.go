package mood

import (
	"strings"
)

// Rules reads mood from the shape of a finished command: how it ended, how
// long it took, what kind of command it was, how many in a row failed, and the
// hour. It needs no model and answers instantly. Events that aren't commands
// fall back to keyword matching.
type Rules struct{}

type cmdKind int

const (
	kindOther cmdKind = iota
	kindPush
	kindCommit
	kindTest
	kindBuild
	kindDeploy
	kindEdit
	kindInstall
	kindLook
	kindInvestigate
)

var (
	pushPrefixes   = []string{"git push", "git merge", "git tag", "gh pr merge", "gh release"}
	deployMarkers  = []string{"deploy", "kubectl apply", "terraform apply", "helm upgrade", "helm install", "fly launch", "vercel", "ansible-playbook"}
	buildPrefixes  = []string{"go build", "go install", "cargo build", "make", "mvn", "gradle", "tsc", "webpack", "vite build", "xcodebuild", "docker build", "swift build", "npm run build", "yarn build", "pnpm build", "cmake"}
	testMarkers    = []string{"test", "pytest", "jest", "vitest", "rspec", "mocha", "go vet", "lint"}
	editPrefixes   = []string{"vim", "nvim", "vi ", "nano", "emacs", "code ", "hx ", "micro"}
	installPrefix  = []string{"npm install", "npm i ", "yarn add", "pip install", "pip3 install", "brew install", "go get", "cargo add", "apt install", "apt-get install", "pnpm add", "bundle install"}
	lookPrefixes   = []string{"git status", "git log", "git branch", "cat ", "less ", "head ", "tail ", "which ", "man ", "docker ps", "kubectl get", "ps ", "top", "df ", "du ", "echo ", "date", "whoami"}
	investigate    = []string{"git diff", "git show", "git blame", "grep ", "rg ", "find ", "ag "}
	interruptCodes = map[int]bool{130: true, 131: true, 143: true}
)

func hasPrefix(cmd string, prefixes []string) bool {
	for _, p := range prefixes {
		if cmd == strings.TrimSpace(p) || strings.HasPrefix(cmd, p) {
			return true
		}
	}
	return false
}

func hasMarker(cmd string, markers []string) bool {
	for _, m := range markers {
		if strings.Contains(cmd, m) {
			return true
		}
	}
	return false
}

func classify(cmd string) cmdKind {
	c := strings.ToLower(strings.TrimSpace(cmd))
	c = strings.TrimPrefix(c, "sudo ")
	words := strings.Fields(c)
	head := c
	if len(words) > 3 {
		head = strings.Join(words[:3], " ")
	}
	switch {
	case strings.HasPrefix(c, "git commit"):
		return kindCommit
	case hasPrefix(c, pushPrefixes):
		return kindPush
	case hasMarker(head, deployMarkers):
		return kindDeploy
	case hasMarker(head, testMarkers):
		return kindTest
	case hasPrefix(c, buildPrefixes):
		return kindBuild
	case hasPrefix(c, editPrefixes):
		return kindEdit
	case hasPrefix(c, installPrefix):
		return kindInstall
	case hasPrefix(c, investigate):
		return kindInvestigate
	case hasPrefix(c, lookPrefixes):
		return kindLook
	}
	return kindOther
}

func (Rules) Read(e Event) (Reading, error) {
	if e.Kind != "command" {
		return Keyword{}.Read(e)
	}
	s := map[Mood]float64{Calm: 0.25, Focused: 0.15}
	add := func(m Mood, w float64) { s[m] += w }
	kind := classify(e.Cmd)

	switch {
	case interruptCodes[e.Exit]:
		// Stopped on purpose, so not a failure.
		add(Calm, 0.6)
		add(Focused, 0.3)

	case e.Exit == 127:
		// Command not found: a typo, annoying but mild.
		add(Frustrated, 0.7)
		add(Calm, 0.3)

	case e.Exit != 0:
		streak := e.FailStreak
		if streak > 4 {
			streak = 4
		}
		add(Frustrated, 1.0+0.25*float64(streak))

	default:
		switch kind {
		case kindPush, kindDeploy:
			add(Celebrating, 1.0)
		case kindTest:
			add(Celebrating, 0.7)
			add(Focused, 0.2)
		case kindBuild:
			add(Celebrating, 0.35)
			add(Focused, 0.35)
		case kindCommit:
			add(Focused, 0.5)
			add(Celebrating, 0.3)
		case kindEdit:
			add(Focused, 0.8)
		case kindInstall:
			add(Calm, 0.3)
			add(Focused, 0.3)
		case kindLook:
			add(Calm, 0.4)
			add(Focused, 0.2)
		case kindInvestigate:
			add(Focused, 0.6)
			add(Calm, 0.1)
		default:
			add(Calm, 0.5)
			add(Focused, 0.3)
		}
		if e.FailStreak >= 2 {
			// Relief after a bad run.
			add(Celebrating, 1.2)
		} else if e.FailStreak == 1 {
			add(Celebrating, 0.5)
		}
		if e.Millis >= 60_000 {
			add(Focused, 0.4)
			add(Celebrating, 0.2)
		}
	}

	switch {
	case e.Hour >= 0 && e.Hour < 5:
		if e.Exit != 0 {
			add(Tired, 0.5)
		} else {
			add(Tired, 1.1)
		}
	case e.Hour >= 23:
		add(Tired, 0.4)
	}

	// Words in a commit message, or in a command we have no rule for, can nudge
	// the mood. Commands the rules already understand are not double counted.
	if kind == kindCommit || kind == kindOther {
		kw, _ := Keyword{}.Read(Event{Text: e.Cmd})
		if top, conf := kw.Top(); conf > 0.4 && top != Calm {
			add(top, 0.8*conf)
		}
	}

	total := 0.0
	for _, v := range s {
		total += v
	}
	scores := make(map[Mood]float64, len(All))
	for _, m := range All {
		scores[m] = s[m] / total
	}
	return Reading{Scores: scores}, nil
}
