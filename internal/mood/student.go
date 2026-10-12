package mood

import (
	_ "embed"
	"encoding/json"
	"hash/fnv"
	"math"
	"strconv"
	"strings"
)

//go:embed student_model.json
var embeddedStudent []byte

// Student is a small learned classifier for command events. It was trained to
// imitate a larger decision model, and it answers in microseconds with no
// server. It is softmax regression over hashed features of the command, how it
// ended, the failure streak and the hour.
type Student struct {
	Dim  int
	Bias [5]float64
	W    map[int][5]float64
}

// LoadStudent returns the student built into the program, or nil if none has
// been trained yet.
func LoadStudent() *Student {
	s, err := ParseStudent(embeddedStudent)
	if err != nil || s.Dim == 0 {
		return nil
	}
	return s
}

type studentFile struct {
	Dim  int                  `json:"dim"`
	Bias []float32            `json:"bias"`
	W    map[string][]float32 `json:"w"`
}

func ParseStudent(data []byte) (*Student, error) {
	var f studentFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	s := &Student{Dim: f.Dim, W: make(map[int][5]float64, len(f.W))}
	for i := 0; i < len(f.Bias) && i < 5; i++ {
		s.Bias[i] = float64(f.Bias[i])
	}
	for k, v := range f.W {
		idx, err := strconv.Atoi(k)
		if err != nil || len(v) != 5 {
			continue
		}
		var w [5]float64
		for i := range w {
			w[i] = float64(v[i])
		}
		s.W[idx] = w
	}
	return s, nil
}

// Marshal writes the student, dropping weights too small to matter.
func (s *Student) Marshal() ([]byte, error) {
	f := studentFile{Dim: s.Dim, W: map[string][]float32{}}
	for _, b := range s.Bias {
		f.Bias = append(f.Bias, float32(b))
	}
	for idx, w := range s.W {
		big := false
		for _, v := range w {
			if math.Abs(v) > 0.02 {
				big = true
			}
		}
		if !big {
			continue
		}
		row := make([]float32, 5)
		for i := range row {
			row[i] = float32(math.Round(w[i]*1000) / 1000)
		}
		f.W[strconv.Itoa(idx)] = row
	}
	return json.Marshal(f)
}

func (s *Student) logits(feats []int) [5]float64 {
	z := s.Bias
	for _, f := range feats {
		if w, ok := s.W[f]; ok {
			for k := range z {
				z[k] += w[k]
			}
		}
	}
	return z
}

func softmax(z [5]float64) [5]float64 {
	max := z[0]
	for _, v := range z {
		if v > max {
			max = v
		}
	}
	var out [5]float64
	sum := 0.0
	for i, v := range z {
		out[i] = math.Exp(v - max)
		sum += out[i]
	}
	for i := range out {
		out[i] /= sum
	}
	return out
}

// Read answers command events. Anything else goes to the rules, which fall back
// to keywords.
func (s *Student) Read(e Event) (Reading, error) {
	if e.Kind != "command" {
		return Rules{}.Read(e)
	}
	p := softmax(s.logits(Features(e, s.Dim)))
	scores := make(map[Mood]float64, len(All))
	for i, m := range All {
		scores[m] = p[i]
	}
	return Reading{Scores: scores}, nil
}

func hashFeature(name string, dim int) int {
	h := fnv.New32a()
	h.Write([]byte(name))
	return int(h.Sum32() % uint32(dim))
}

func exitClass(code int) string {
	switch {
	case code == 0:
		return "ok"
	case code == 127:
		return "notfound"
	case code == 130 || code == 131 || code == 143:
		return "interrupt"
	case code >= 128:
		return "killed"
	}
	return "fail"
}

func durationBucket(ms int64) string {
	switch {
	case ms < 1000:
		return "instant"
	case ms < 10_000:
		return "short"
	case ms < 60_000:
		return "medium"
	case ms < 300_000:
		return "long"
	}
	return "verylong"
}

func hourBucket(h int) string {
	switch {
	case h >= 0 && h < 5:
		return "night"
	case h >= 23:
		return "late"
	case h < 12:
		return "morning"
	case h < 18:
		return "afternoon"
	}
	return "evening"
}

func streakBucket(n int) string {
	switch {
	case n <= 0:
		return "0"
	case n == 1:
		return "1"
	case n == 2:
		return "2"
	case n <= 4:
		return "3-4"
	}
	return "5+"
}

var kindNames = map[cmdKind]string{
	kindOther: "other", kindPush: "push", kindCommit: "commit", kindTest: "test",
	kindBuild: "build", kindDeploy: "deploy", kindEdit: "edit", kindInstall: "install",
	kindLook: "look", kindInvestigate: "investigate",
}

// Features turns a command event into hashed feature indices.
func Features(e Event, dim int) []int {
	ex, dur, hr, st := exitClass(e.Exit), durationBucket(e.Millis), hourBucket(e.Hour), streakBucket(e.FailStreak)
	kind := kindNames[classify(e.Cmd)]

	names := []string{
		"exit=" + ex, "dur=" + dur, "hour=" + hr, "streak=" + st, "kind=" + kind,
		"exit+kind=" + ex + "+" + kind,
		"exit+streak=" + ex + "+" + st,
		"exit+hour=" + ex + "+" + hr,
		"exit+dur=" + ex + "+" + dur,
		"kind+hour=" + kind + "+" + hr,
		"exit+kind+streak=" + ex + "+" + kind + "+" + st,
	}
	words := strings.Fields(strings.ToLower(e.Cmd))
	if len(words) > 0 {
		names = append(names, "first="+words[0], "exit+first="+ex+"+"+words[0])
	}
	if len(words) > 1 {
		names = append(names, "first2="+words[0]+" "+words[1])
	}
	for _, w := range words {
		w = strings.Trim(w, `"'.,:;!?()[]`)
		if len(w) >= 2 && !strings.HasPrefix(w, "-") && !strings.ContainsAny(w, "/=.") {
			names = append(names, "w="+w, "exit+w="+ex+"+"+w)
		}
	}

	seen := make(map[int]bool, len(names))
	out := make([]int, 0, len(names))
	for _, n := range names {
		i := hashFeature(n, dim)
		if !seen[i] {
			seen[i] = true
			out = append(out, i)
		}
	}
	return out
}

// Sample is one training example: an event and the teacher's distribution over
// moods, in the order of All.
type Sample struct {
	Event  Event
	Target [5]float64
}

// TrainStudent fits a student to the samples by stochastic gradient descent on
// the cross-entropy between its output and the teacher's distribution.
func TrainStudent(samples []Sample, dim, epochs int, lr, l2 float64, seed int64) *Student {
	dense := make([][5]float64, dim)
	var bias [5]float64
	feats := make([][]int, len(samples))
	for i, s := range samples {
		feats[i] = Features(s.Event, dim)
	}

	order := make([]int, len(samples))
	for i := range order {
		order[i] = i
	}
	rng := newLCG(seed)
	for ep := 0; ep < epochs; ep++ {
		rng.shuffle(order)
		rate := lr / (1 + 0.15*float64(ep))
		for _, i := range order {
			z := bias
			for _, f := range feats[i] {
				for k := range z {
					z[k] += dense[f][k]
				}
			}
			p := softmax(z)
			for k := range p {
				g := p[k] - samples[i].Target[k]
				bias[k] -= rate * g
				for _, f := range feats[i] {
					dense[f][k] -= rate * (g + l2*dense[f][k])
				}
			}
		}
	}

	s := &Student{Dim: dim, Bias: bias, W: map[int][5]float64{}}
	for i, w := range dense {
		for _, v := range w {
			if v != 0 {
				s.W[i] = w
				break
			}
		}
	}
	return s
}

// lcg is a tiny deterministic shuffler, so training is repeatable.
type lcg struct{ state uint64 }

func newLCG(seed int64) *lcg {
	return &lcg{state: uint64(seed)*6364136223846793005 + 1442695040888963407}
}

func (l *lcg) next() uint64 {
	l.state = l.state*6364136223846793005 + 1442695040888963407
	return l.state >> 33
}

func (l *lcg) shuffle(a []int) {
	for i := len(a) - 1; i > 0; i-- {
		j := int(l.next() % uint64(i+1))
		a[i], a[j] = a[j], a[i]
	}
}
