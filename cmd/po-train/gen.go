package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
)

// Scenario is one simulated command.
type Scenario struct {
	Cmd    string `json:"cmd"`
	Exit   int    `json:"exit"`
	Ms     int64  `json:"ms"`
	Streak int    `json:"streak"`
	Hour   int    `json:"hour"`
}

type category struct {
	name     string
	weight   float64
	failRate float64 // chance of failing when nothing has gone wrong yet
	stuck    float64 // chance of failing again right after a failure
	minMs    int64
	maxMs    int64
	cmds     []string
}

var pick = func(r *rand.Rand, s []string) string { return s[r.Intn(len(s))] }

var packages = []string{"./...", "./internal/...", "./cmd/...", "./pkg/server", "./api"}
var files = []string{"main.go", "handler.go", "app.py", "index.ts", "server.js", "README.md", "notes.md", "config.yaml", "Makefile", "lib.rs"}
var names = []string{"api", "web", "worker", "auth", "billing", "search", "ui", "parser", "cache", "queue"}
var messages = []string{
	"fix typo", "wip", "wip: pagination", "refactor parser", "add tests", "update deps", "fix the fix",
	"finally works", "please work", "ugh", "revert", "ship it", "final final v2", "cleanup",
	"extract config loading", "fix flaky test", "going to bed", "good enough for today", "lunch break",
	"hotfix", "bump version", "document the api", "remove dead code", "it works now", "stop, going to bed",
	"why", "fix build again", "tomorrow's problem", "first draft", "tidy imports",
}

var categories = []category{
	{"test", 18, 0.25, 0.55, 800, 120_000, nil},
	{"build", 12, 0.2, 0.5, 2_000, 400_000, nil},
	{"git", 20, 0.08, 0.2, 40, 4_000, nil},
	{"deploy", 4, 0.18, 0.45, 5_000, 300_000, nil},
	{"run", 8, 0.15, 0.4, 500, 600_000, nil},
	{"edit", 8, 0.0, 0.0, 60_000, 7_200_000, nil},
	{"look", 20, 0.06, 0.2, 20, 900, nil},
	{"install", 5, 0.12, 0.35, 1_500, 90_000, nil},
	{"typo", 3, 1.0, 0.3, 5, 30, nil},
	{"other", 6, 0.1, 0.3, 30, 20_000, nil},
}

func command(r *rand.Rand, cat string) string {
	switch cat {
	case "test":
		return pick(r, []string{
			"go test " + pick(r, packages), "go test -race " + pick(r, packages), "pytest", "pytest tests/",
			"pytest tests/test_" + pick(r, names) + ".py -k " + pick(r, names), "npm test", "cargo test",
			"jest", "vitest run", "rspec", "mvn test", "make test", "go vet ./...", "npm run lint",
		})
	case "build":
		return pick(r, []string{
			"go build " + pick(r, packages), "cargo build --release", "npm run build", "make", "tsc --noEmit",
			"docker build -t " + pick(r, names) + " .", "swift build", "gradle build", "xcodebuild", "cmake --build .",
		})
	case "git":
		return pick(r, []string{
			"git status", "git add -A", `git commit -m "` + pick(r, messages) + `"`, `git commit -m "` + pick(r, messages) + `"`,
			"git push", "git push origin main", "git pull", "git merge " + pick(r, names), "git rebase main",
			"git checkout " + pick(r, names), "git stash", "git log --oneline", "git diff", "git fetch", "gh pr create",
			"gh pr merge 42 --squash", "git tag v1.0.0",
		})
	case "deploy":
		return pick(r, []string{
			"make deploy", "kubectl apply -f deploy.yaml", "terraform apply -auto-approve", "fly deploy",
			"vercel --prod", "helm upgrade " + pick(r, names) + " ./chart", "ansible-playbook site.yml",
		})
	case "run":
		return pick(r, []string{
			"python app.py", "node server.js", "go run ./cmd/server", "cargo run", "npm start", "npm run dev",
			"./bin/" + pick(r, names), "python manage.py runserver", "uvicorn main:app --reload",
		})
	case "edit":
		return pick(r, []string{"vim " + pick(r, files), "nvim " + pick(r, files), "nano " + pick(r, files), "code .", "hx " + pick(r, files)})
	case "look":
		return pick(r, []string{
			"cat " + pick(r, files), "grep -rn TODO .", `rg "` + pick(r, names) + `" internal/`, "find . -name " + pick(r, files),
			"tail -f app.log", "docker ps", "kubectl get pods", "curl -f https://api.example.com/health",
			"which python3", "echo done", "date", "man tar", "ps aux", "df -h",
		})
	case "install":
		return pick(r, []string{
			"npm install", "pip install " + pick(r, names), "brew install " + pick(r, names), "go get " + pick(r, names),
			"cargo add " + pick(r, names), "yarn add " + pick(r, names), "bundle install",
		})
	case "typo":
		return pick(r, []string{"gti status", "pytst", "sl", "cd..", "gp push", "grpe foo", "dcoker ps", "claer"})
	}
	return pick(r, []string{
		"chmod +x " + pick(r, files), "mkdir " + pick(r, names), "rm -r build/", "cp " + pick(r, files) + " backup/",
		"ssh " + pick(r, names) + ".example.com", "scp " + pick(r, files) + " host:", "sudo systemctl restart " + pick(r, names),
		"open .", "touch " + pick(r, files),
	})
}

func failureExit(r *rand.Rand, cat string) int {
	switch cat {
	case "typo":
		return 127
	case "git":
		return pick2(r, 1, 128)
	case "build", "deploy":
		return pick2(r, 1, 2, 1, 1, 137)
	}
	return pick2(r, 1, 1, 1, 2)
}

func pick2(r *rand.Rand, codes ...int) int { return codes[r.Intn(len(codes))] }

func sessionHour(r *rand.Rand) int {
	switch x := r.Float64(); {
	case x < 0.07:
		return r.Intn(5) // small hours
	case x < 0.12:
		return 23
	case x < 0.22:
		return 5 + r.Intn(4)
	case x < 0.80:
		return 9 + r.Intn(9)
	}
	return 18 + r.Intn(5)
}

func weighted(r *rand.Rand) category {
	total := 0.0
	for _, c := range categories {
		total += c.weight
	}
	x := r.Float64() * total
	for _, c := range categories {
		if x < c.weight {
			return c
		}
		x -= c.weight
	}
	return categories[0]
}

func logUniform(r *rand.Rand, lo, hi int64) int64 {
	if hi <= lo {
		return lo
	}
	span := float64(hi) / float64(lo)
	return int64(float64(lo) * math.Pow(span, r.Float64()))
}

func runGen(args []string) {
	fs := flag.NewFlagSet("gen", flag.ExitOnError)
	n := fs.Int("n", 2500, "number of commands")
	seed := fs.Int64("seed", 1, "random seed")
	out := fs.String("out", "data/scenarios.jsonl", "output file")
	fs.Parse(args)

	r := rand.New(rand.NewSource(*seed))
	if err := os.MkdirAll("data", 0o755); err != nil {
		fatal(err)
	}
	f, err := os.Create(*out)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	written, fails := 0, 0
	for written < *n {
		hour := sessionHour(r)
		streak := 0
		last := ""
		lastCat := category{}
		for i, length := 0, 8+r.Intn(30); i < length && written < *n; i++ {
			var cat category
			var cmd string
			if streak > 0 && last != "" && r.Float64() < 0.6 {
				cat, cmd = lastCat, last // run the thing that just failed again
			} else {
				cat = weighted(r)
				cmd = command(r, cat.name)
			}
			p := cat.failRate
			if streak > 0 {
				p = cat.stuck
			}
			exit := 0
			if r.Float64() < p {
				exit = failureExit(r, cat.name)
			} else if cat.name == "run" && r.Float64() < 0.3 {
				exit = 130
			}
			s := Scenario{Cmd: cmd, Exit: exit, Ms: logUniform(r, cat.minMs, cat.maxMs), Streak: streak, Hour: hour}
			b, _ := json.Marshal(s)
			w.Write(append(b, '\n'))
			written++
			switch {
			case exit == 0:
				streak = 0
			case exit < 128:
				streak++
				fails++
			}
			last, lastCat = cmd, cat
			if r.Float64() < 0.03 {
				hour = (hour + 1) % 24
			}
		}
	}
	fmt.Printf("wrote %d scenarios to %s (%d failures)\n", written, *out, fails)
}
