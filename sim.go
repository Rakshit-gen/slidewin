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
}

// SimReport is what the PoP did with the traffic.
type SimReport struct {
	Requests int
	Blocked  int
	// Leaked counts requests served while the client's exact rate, counting
	// every request it sent, was over the limit.
	Leaked int
	// Collateral counts requests blocked while the client's exact rate was at
	// or under the limit. Most come from the mitigation timeout outlasting the
	// burst that caused it.
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
	log := NewLog(cfg.Rule.Window)
	var r SimReport
	var flushed time.Time
	feed(func(q Request) {
		clk.t = q.At
		if q.At.Sub(flushed) >= cfg.Lag {
			pop.Flush()
			flushed = q.At
		}
		served := pop.Allow(q.Conn, q.Client)
		log.Add(q.Client, q.At)
		over := log.Rate(q.Client, q.At) > int(cfg.Rule.Limit)
		r.Requests++
		switch {
		case !served:
			r.Blocked++
			if !over {
				r.Collateral++
			}
		case over:
			r.Leaked++
		}
	})
	r.RoundTrips = pop.Store.RoundTrips()
	return r
}
