package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/bkodzo/po/internal/event"
	"github.com/bkodzo/po/internal/mood"
)

// showMisses makes eval list every line a reader got wrong.
var showMisses bool

type labelled struct {
	want  mood.Mood
	text  string
	event mood.Event
}

func loadLabelled(path string) ([]labelled, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []labelled
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 6)
		switch len(fields) {
		case 2:
			// label, text
			out = append(out, labelled{mood.Mood(fields[0]), fields[1], mood.Event{Kind: "text", Text: fields[1]}})
		case 6:
			// label, exit, ms, failure streak, hour, command
			exit, _ := strconv.Atoi(fields[1])
			ms, _ := strconv.ParseInt(fields[2], 10, 64)
			streak, _ := strconv.Atoi(fields[3])
			hour, _ := strconv.Atoi(fields[4])
			c := event.Command{Text: fields[5], Exit: exit, Millis: ms}
			out = append(out, labelled{mood.Mood(fields[0]), fields[5], c.ToEventAt(hour, streak)})
		}
	}
	return out, scanner.Err()
}

type result struct {
	got  mood.Mood
	conf float64
	err  error
}

func runEval(path string, readers map[string]mood.Reader, order []string) error {
	lines, err := loadLabelled(path)
	if err != nil {
		return err
	}
	for _, name := range order {
		results := readAll(readers[name], lines)
		report(name, lines, results)
	}
	return nil
}

func readAll(r mood.Reader, lines []labelled) []result {
	results := make([]result, len(lines))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 1)
	for i, l := range lines {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			reading, err := r.Read(l.event)
			if err != nil {
				results[i] = result{err: err}
				return
			}
			m, c := reading.Top()
			results[i] = result{got: m, conf: c}
		}()
	}
	wg.Wait()
	return results
}

func report(name string, lines []labelled, results []result) {
	correct, failed := 0, 0
	perMood := map[mood.Mood][2]int{}
	for i, l := range lines {
		if results[i].err != nil {
			failed++
			continue
		}
		c := perMood[l.want]
		c[1]++
		if results[i].got == l.want {
			correct++
			c[0]++
		} else if showMisses {
			fmt.Printf("  miss: want %-11s got %-11s %.2f  %s\n", l.want, results[i].got, results[i].conf, l.event.Text)
		}
		perMood[l.want] = c
	}
	scored := len(lines) - failed
	for i, l := range lines {
		if results[i].err != nil {
			fmt.Printf("  first error on %q: %v\n", l.text, results[i].err)
			break
		}
	}
	fmt.Printf("\n%s: %d/%d correct (%d errors)\n", name, correct, scored, failed)
	for _, m := range mood.All {
		c := perMood[m]
		fmt.Printf("  %-12s %2d/%2d\n", m, c[0], c[1])
	}
	fmt.Println("  cutoff  kept  accuracy of kept")
	for _, t := range []float64{0.2, 0.25, 0.3, 0.35, 0.4, 0.45, 0.5, 0.6} {
		kept, right := 0, 0
		for i, l := range lines {
			if results[i].err != nil || results[i].conf < t {
				continue
			}
			kept++
			if results[i].got == l.want {
				right++
			}
		}
		if kept == 0 {
			fmt.Printf("  %.2f    0/%d\n", t, scored)
			continue
		}
		fmt.Printf("  %.2f    %2d/%d  %3.0f%%\n", t, kept, scored, 100*float64(right)/float64(kept))
	}
}
