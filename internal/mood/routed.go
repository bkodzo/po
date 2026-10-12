package mood

import "sync"

// RouteStats counts how events were answered.
type RouteStats struct {
	Fast       int // answered by the fast layer alone
	Slow       int // answered by the model
	SlowFailed int // the model was asked but could not answer, so the fast answer was used
}

// Routed answers with a fast reader when it is sure, and asks a slower, smarter
// reader only when it is not. If the slow reader fails, the fast answer stands,
// so a missing model never leaves the pet without a reaction.
type Routed struct {
	Fast Reader
	Slow Reader // may be nil
	Gate float64

	mu    sync.Mutex
	stats RouteStats
}

func (r *Routed) Stats() RouteStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stats
}

func (r *Routed) count(f func(*RouteStats)) {
	r.mu.Lock()
	f(&r.stats)
	r.mu.Unlock()
}

func (r *Routed) Read(e Event) (Reading, error) {
	fast, fastErr := r.Fast.Read(e)
	if fastErr == nil {
		if _, conf := fast.Top(); conf >= r.Gate || r.Slow == nil {
			r.count(func(s *RouteStats) { s.Fast++ })
			return fast, nil
		}
	}
	if r.Slow != nil {
		if slow, err := r.Slow.Read(e); err == nil {
			r.count(func(s *RouteStats) { s.Slow++ })
			return slow, nil
		}
		r.count(func(s *RouteStats) { s.SlowFailed++ })
	}
	if fastErr == nil {
		return fast, nil
	}
	return Reading{}, fastErr
}
