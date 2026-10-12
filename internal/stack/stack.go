// Package stack turns the settings into the reader Po uses.
package stack

import (
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/bkodzo/po/internal/config"
	"github.com/bkodzo/po/internal/mood"
)

// modelTimeout bounds one question to the model. It is short so that a slow or
// stuck model cannot hold up the pet's reactions.
const modelTimeout = 8 * time.Second

type Stack struct {
	Reader mood.Reader
	// Routed is set when the reader is auto, so its counters can be shown.
	Routed *mood.Routed
	// MinConfidence is how sure a reading must be before the pet acts on it.
	MinConfidence float64
	Cfg           config.Config
}

func Build(cfg config.Config) *Stack {
	s := &Stack{Cfg: cfg, MinConfidence: 0.25}
	switch cfg.Reader {
	case "keyword":
		s.Reader = mood.Keyword{}
		s.MinConfidence = 0.45
	case "rules":
		s.Reader = mood.Rules{}
	case "decision":
		s.Reader = mood.Fallback{Primary: decision(cfg), Backup: mood.Rules{}}
	case "ollama":
		o := mood.NewOllama(cfg.URL, cfg.Model)
		o.Client.Timeout = modelTimeout
		s.Reader = mood.Fallback{Primary: o, Backup: mood.Rules{}}
	default:
		var slow mood.Reader
		if cfg.URL != "" {
			slow = decision(cfg)
		}
		s.Routed = &mood.Routed{Fast: mood.Rules{}, Slow: slow, Gate: cfg.Gate}
		s.Reader = s.Routed
	}
	return s
}

func decision(cfg config.Config) *mood.Decision {
	d := mood.NewDecision(cfg.URL, cfg.Model)
	d.Key = cfg.ReadKey()
	d.Client = &http.Client{Timeout: modelTimeout}
	return d
}

// UsesModel reports whether the settings include a model at all.
func (s *Stack) UsesModel() bool {
	switch s.Cfg.Reader {
	case "keyword", "rules":
		return false
	}
	return s.Cfg.URL != ""
}

// IsLocal reports whether the model address is on this machine.
func IsLocal(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Reachable reports whether something is listening at the model address.
func Reachable(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(u.Hostname(), port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
