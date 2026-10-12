// Package stack turns the settings into the reader Po uses.
package stack

import (
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/bkodzo/po/internal/config"
	"github.com/bkodzo/po/internal/ipc"
	"github.com/bkodzo/po/internal/modelserver"
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
	// Managed starts and stops the local model server on demand. It is nil when
	// Po is not asked to manage one, and ManagedErr says why if it could not.
	Managed    *modelserver.Supervisor
	ManagedErr error
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
		s.Reader = mood.Fallback{Primary: s.model(cfg), Backup: mood.Rules{}}
	case "ollama":
		o := mood.NewOllama(cfg.URL, cfg.Model)
		o.Client.Timeout = modelTimeout
		s.Reader = mood.Fallback{Primary: o, Backup: mood.Rules{}}
	default:
		var slow mood.Reader
		if cfg.URL != "" {
			slow = s.model(cfg)
		}
		s.Routed = &mood.Routed{Fast: mood.Rules{}, Slow: slow, Gate: cfg.Gate}
		s.Reader = s.Routed
	}
	return s
}

// model is the decision reader, wrapped so Po starts the local server when it is
// wanted if a model file is configured.
func (s *Stack) model(cfg config.Config) mood.Reader {
	d := decision(cfg)
	if cfg.LocalModel == "" || !IsLocal(cfg.URL) {
		return d
	}
	pidFile := ""
	if sock, err := ipc.Path(); err == nil {
		pidFile = filepath.Join(filepath.Dir(sock), "model.pid")
	}
	sup, err := modelserver.New("", cfg.LocalModel, cfg.URL, pidFile, time.Duration(cfg.IdleMinutes)*time.Minute)
	if err != nil {
		s.ManagedErr = err
		return d
	}
	s.Managed = sup
	return modelserver.Managed{Inner: d, Sup: sup}
}

// Close stops anything Po started.
func (s *Stack) Close() {
	if s.Managed != nil {
		s.Managed.Stop()
	}
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
