package slidewin

import "time"

// SimConfig sets up a PoP simulation.
type SimConfig struct {
	Rule    Rule
	Servers int
	// Lag is how much simulated time passes between counting runs. Requests
	// in between are served on the old mitigation state.
	Lag     time.Duration
	NoCache bool
	// KillAt, if set, takes the first server's shard down at that moment. It
	// stays down; the cluster ejects it and its clients restart from zero on
	// the shards that take over.
	KillAt time.Time
}

// SimReport is what the PoP did with the traffic.
type SimReport struct {
	Requests int
	Blocked  int
	// Leaked counts requests served that took the client over the limit:
	// with them, the client had more than Limit requests served in the last
	// window.
	Leaked int
	// Collateral counts requests blocked while the client was sending at or
	// under the limit, counting everything it sent in the last window. Most
	// come from the mitigation timeout outlasting the burst that caused it.
	Collateral int
	RoundTrips int64
}

// Simulate replays the requests that feed produces through a whole PoP.
func Simulate(cfg SimConfig, feed func(func(Request))) SimReport {
	clk := &clock{}
	pop := NewPoP(cfg.Servers, cfg.Rule, clk.now)
	for _, s := range pop.Servers {
		s.NoCache = cfg.NoCache
	}
	sent := NewLog(cfg.Rule.Window)   // every request
	served := NewLog(cfg.Rule.Window) // requests the PoP let through
	var r SimReport
	var flushed time.Time
	killed := false
	feed(func(q Request) {
		clk.t = q.At
		if !cfg.KillAt.IsZero() && !killed && !q.At.Before(cfg.KillAt) {
			pop.Store.Node("edge01").SetDown(true)
			killed = true
		}
		if q.At.Sub(flushed) >= cfg.Lag {
			pop.Flush()
			flushed = q.At
		}
		limit := int(cfg.Rule.Limit)
		sent.Add(q.Client, q.At)
		r.Requests++
		if !pop.Allow(q.Conn, q.Client) {
			r.Blocked++
			if sent.Rate(q.Client, q.At) <= limit {
				r.Collateral++
			}
			return
		}
		served.Add(q.Client, q.At)
		if served.Rate(q.Client, q.At) > limit {
			r.Leaked++
		}
	})
	r.RoundTrips = pop.Store.RoundTrips()
	return r
}
