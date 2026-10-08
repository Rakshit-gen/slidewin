package slidewin

import (
	"errors"
	"fmt"
	"time"
)

// Rule is a limit such as 50 requests per minute. A client that goes over is
// mitigated for Timeout, or for one Window if Timeout is zero.
type Rule struct {
	Limit   int64
	Window  time.Duration
	Timeout time.Duration
}

func (r Rule) timeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return r.Window
}

// Limiter counts requests per client in a Cluster and judges them against Rule.
type Limiter struct {
	rule  Rule
	store *Cluster
	now   func() time.Time
}

// NewLimiter returns a limiter for rule backed by store.
func NewLimiter(rule Rule, store *Cluster, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{rule: rule, store: store, now: now}
}

func counterKey(client string, index int64) string {
	return fmt.Sprintf("rl:{%s}:%d", client, index)
}

// Count records one request from client in the current window. INCR fails on a
// missing counter, so the first request of a window falls back to SET. Two
// servers can both miss and both SET, losing one count; the blog accepts this
// rather than use CAS, which fails constantly under attack.
func (l *Limiter) Count(client string) {
	i, _ := Bucket(l.now(), l.rule.Window)
	key := counterKey(client, i)
	if _, err := l.store.Incr(key, 1); errors.Is(err, ErrMiss) {
		// The counter is read during the next window, then never again.
		l.store.Set(key, 1, 2*l.rule.Window)
	}
}

// Rate is the sliding window estimate of client's requests over the last window.
func (l *Limiter) Rate(client string) float64 {
	i, elapsed := Bucket(l.now(), l.rule.Window)
	prev, cur := counterKey(client, i-1), counterKey(client, i)
	got := l.store.GetMulti(prev, cur)
	return Estimate(got[prev], got[cur], elapsed, l.rule.Window)
}

func mitigationKey(client string) string { return "mit:{" + client + "}" }

// Judge compares client's rate with the limit and, when it is over, stores a
// mitigation flag that every server in the PoP will see. The flag holds the
// time the mitigation ends and expires on its own at that time.
func (l *Limiter) Judge(client string) bool {
	if l.Rate(client) <= float64(l.rule.Limit) {
		return false
	}
	d := l.rule.timeout()
	l.store.Set(mitigationKey(client), l.now().Add(d).UnixNano(), d)
	return true
}

// Mitigated reports whether client is under a mitigation, and until when.
func (l *Limiter) Mitigated(client string) (time.Time, bool) {
	end, err := l.store.Get(mitigationKey(client))
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, end), true
}
