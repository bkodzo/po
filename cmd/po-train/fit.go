package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/bkodzo/po/internal/event"
	"github.com/bkodzo/po/internal/mood"
)

func argmax(p [5]float64) int {
	best := 0
	for i, v := range p {
		if v > p[best] {
			best = i
		}
	}
	return best
}

func target(l Labelled, sharpen float64) [5]float64 {
	var t [5]float64
	sum := 0.0
	for i, m := range mood.All {
		t[i] = math.Pow(l.Scores[string(m)], sharpen)
		sum += t[i]
	}
	for i := range t {
		t[i] /= sum
	}
	return t
}

func topOf(r mood.Reading) (int, float64) {
	m, c := r.Top()
	for i, x := range mood.All {
		if x == m {
			return i, c
		}
	}
	return 0, c
}

// agreement scores a reader against the teacher on held-out scenarios, overall
// and for the answers it is most sure of.
func agreement(name string, r mood.Reader, held []Labelled) {
	type row struct{ agree, conf bool }
	var rows []struct {
		ok   bool
		conf float64
	}
	for _, l := range held {
		reading, err := r.Read(l.S.event())
		if err != nil {
			continue
		}
		got, conf := topOf(reading)
		want := argmax(target(l, 1))
		rows = append(rows, struct {
			ok   bool
			conf float64
		}{got == want, conf})
	}
	ok := 0
	for _, r := range rows {
		if r.ok {
			ok++
		}
	}
	fmt.Printf("  %-8s agrees with the teacher on %d/%d (%.0f%%)", name, ok, len(rows), 100*float64(ok)/float64(len(rows)))
	for _, gate := range []float64{0.6, 0.8} {
		kept, right := 0, 0
		for _, r := range rows {
			if r.conf >= gate {
				kept++
				if r.ok {
					right++
				}
			}
		}
		if kept > 0 {
			fmt.Printf(" | sure>=%.1f: %d%% of events, %.0f%% agree", gate, 100*kept/len(rows), 100*float64(right)/float64(kept))
		}
	}
	fmt.Println()
}

type handLine struct {
	want  mood.Mood
	event mood.Event
}

func loadHand(path string) []handLine {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []handLine
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		p := strings.SplitN(scanner.Text(), "\t", 6)
		if len(p) != 6 {
			continue
		}
		exit, _ := strconv.Atoi(p[1])
		ms, _ := strconv.ParseInt(p[2], 10, 64)
		streak, _ := strconv.Atoi(p[3])
		hour, _ := strconv.Atoi(p[4])
		out = append(out, handLine{mood.Mood(p[0]), event.Command{Text: p[5], Exit: exit, Millis: ms}.ToEventAt(hour, streak)})
	}
	return out
}

func handAccuracy(name string, r mood.Reader, hand []handLine) {
	ok := 0
	for _, h := range hand {
		reading, err := r.Read(h.event)
		if err != nil {
			continue
		}
		if m, _ := reading.Top(); m == h.want {
			ok++
		}
	}
	fmt.Printf("  %-8s %d/%d (%.0f%%)\n", name, ok, len(hand), 100*float64(ok)/float64(len(hand)))
}

func runFit(args []string) {
	fs := flag.NewFlagSet("fit", flag.ExitOnError)
	in := fs.String("in", "data/labelled.jsonl", "labelled scenarios")
	out := fs.String("out", "", "write the trained student here (internal/mood/student_model.json); empty means report only")
	dim := fs.Int("dim", 4096, "number of hashed features")
	epochs := fs.Int("epochs", 40, "training passes")
	lr := fs.Float64("lr", 0.2, "learning rate")
	l2 := fs.Float64("l2", 1e-4, "weight decay")
	sharpen := fs.Float64("sharpen", 3, "raise the teacher's probabilities to this power so its leaning is clearer")
	hand := fs.String("hand", "testdata/commands.tsv", "hand-labelled commands for a second opinion")
	fs.Parse(args)

	all, err := readLines[Labelled](*in)
	if err != nil || len(all) == 0 {
		fatal(fmt.Errorf("no labelled scenarios in %s (%v)", *in, err))
	}
	var train, held []Labelled
	for _, l := range all {
		if l.I%5 == 0 {
			held = append(held, l)
		} else {
			train = append(train, l)
		}
	}

	samples := func(ls []Labelled) []mood.Sample {
		var s []mood.Sample
		for _, l := range ls {
			s = append(s, mood.Sample{Event: l.S.event(), Target: target(l, *sharpen)})
		}
		return s
	}

	fmt.Printf("%d labelled scenarios: training on %d, holding out %d\n", len(all), len(train), len(held))
	student := mood.TrainStudent(samples(train), *dim, *epochs, *lr, *l2, 1)

	fmt.Println("\nOn the held-out scenarios, against the teacher:")
	agreement("student", student, held)
	agreement("rules", mood.Rules{}, held)
	agreement("keyword", mood.Keyword{}, held)

	if h := loadHand(*hand); len(h) > 0 {
		fmt.Println("\nOn the hand-labelled commands (my own labels; the rules were tuned on these):")
		handAccuracy("student", student, h)
		handAccuracy("rules", mood.Rules{}, h)
	}

	if *out != "" {
		final := mood.TrainStudent(samples(all), *dim, *epochs, *lr, *l2, 1)
		data, err := final.Marshal()
		if err != nil {
			fatal(err)
		}
		if err := os.WriteFile(*out, data, 0o644); err != nil {
			fatal(err)
		}
		fmt.Printf("\nwrote the student (trained on all %d) to %s: %d KB, %d weights\n", len(all), *out, len(data)/1024, len(final.W))
	}
}
