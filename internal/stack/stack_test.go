package stack

import (
	"testing"

	"github.com/bkodzo/po/internal/config"
	"github.com/bkodzo/po/internal/mood"
)

func TestBuildPicksTheRightReader(t *testing.T) {
	for reader, wantRouted := range map[string]bool{"auto": true, "rules": false, "keyword": false, "decision": false, "ollama": false} {
		cfg := config.Default()
		cfg.Reader = reader
		s := Build(cfg)
		if s.Reader == nil {
			t.Fatalf("%s: no reader", reader)
		}
		if (s.Routed != nil) != wantRouted {
			t.Errorf("%s: routed = %v, want %v", reader, s.Routed != nil, wantRouted)
		}
	}
}

func TestKeywordNeedsAHigherCutoff(t *testing.T) {
	cfg := config.Default()
	cfg.Reader = "keyword"
	if Build(cfg).MinConfidence <= Build(config.Default()).MinConfidence {
		t.Error("keyword scores are peaked, so its cutoff should be higher")
	}
}

func TestAutoWithNoURLHasNoModel(t *testing.T) {
	cfg := config.Default()
	cfg.URL = ""
	s := Build(cfg)
	if s.Routed.Slow != nil || s.UsesModel() {
		t.Error("no URL should mean no model")
	}
	if _, err := s.Reader.Read(mood.Event{Kind: "command", Cmd: "go test", Exit: 1, Text: "go test"}); err != nil {
		t.Fatal(err)
	}
}

func TestIsLocal(t *testing.T) {
	for u, want := range map[string]bool{
		"http://127.0.0.1:8080": true, "http://localhost:11434": true, "http://[::1]:8080": true,
		"https://api.typesafe.ai": false, "http://192.168.1.5:8080": false, "": false,
	} {
		if got := IsLocal(u); got != want {
			t.Errorf("IsLocal(%q) = %v, want %v", u, got, want)
		}
	}
}
