package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/bkodzo/po/internal/event"
	"github.com/bkodzo/po/internal/mood"
)

// Labelled is a scenario with the teacher's answer.
type Labelled struct {
	I      int                `json:"i"`
	S      Scenario           `json:"s"`
	Text   string             `json:"text"`
	Scores map[string]float64 `json:"scores"`
}

func (s Scenario) event() mood.Event {
	return event.Command{Text: s.Cmd, Exit: s.Exit, Millis: s.Ms}.ToEventAt(s.Hour, s.Streak)
}

func readLines[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []T
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var v T
		if json.Unmarshal(scanner.Bytes(), &v) == nil {
			out = append(out, v)
		}
	}
	return out, scanner.Err()
}

func runLabel(args []string) {
	fs := flag.NewFlagSet("label", flag.ExitOnError)
	in := fs.String("in", "data/scenarios.jsonl", "scenarios to label")
	out := fs.String("out", "data/labelled.jsonl", "labelled output; an existing file is resumed")
	url := fs.String("url", "http://127.0.0.1:8080", "address of the teacher model")
	modelName := fs.String("model", "clef-flash", "teacher model name")
	workers := fs.Int("workers", 2, "requests in flight at once")
	limit := fs.Int("limit", 0, "label only this many in total (0 means all)")
	fs.Parse(args)

	scenarios, err := readLines[Scenario](*in)
	if err != nil {
		fatal(err)
	}
	done := map[int]bool{}
	if existing, err := readLines[Labelled](*out); err == nil {
		for _, l := range existing {
			done[l.I] = true
		}
	}
	f, err := os.OpenFile(*out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fatal(err)
	}
	defer f.Close()

	teacher := mood.NewDecision(*url, *modelName)
	teacher.Client = &http.Client{Timeout: 90 * time.Second}

	total := len(scenarios)
	if *limit > 0 && *limit < total {
		total = *limit
	}
	todo := make(chan int)
	var mu sync.Mutex
	var wg sync.WaitGroup
	completed, failed := len(done), 0
	start := time.Now()
	newly := 0

	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range todo {
				e := scenarios[i].event()
				var reading mood.Reading
				var err error
				for try := 0; try < 2; try++ {
					if reading, err = teacher.Read(e); err == nil {
						break
					}
					time.Sleep(time.Second)
				}
				mu.Lock()
				if err != nil {
					failed++
					mu.Unlock()
					continue
				}
				scores := map[string]float64{}
				for _, m := range mood.All {
					scores[string(m)] = reading.Scores[m]
				}
				b, _ := json.Marshal(Labelled{I: i, S: scenarios[i], Text: e.Text, Scores: scores})
				f.Write(append(b, '\n'))
				completed++
				newly++
				if newly%50 == 0 {
					rate := float64(newly) / time.Since(start).Seconds()
					left := float64(total-completed) / rate
					fmt.Printf("%d/%d labelled (%.2f/s, about %.0f min left, %d failed)\n", completed, total, rate, left/60, failed)
				}
				mu.Unlock()
			}
		}()
	}
	for i := 0; i < total; i++ {
		if !done[i] {
			todo <- i
		}
	}
	close(todo)
	wg.Wait()
	fmt.Printf("done: %d labelled, %d failed, %s\n", completed, failed, time.Since(start).Round(time.Second))
}
