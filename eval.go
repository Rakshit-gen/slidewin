package slidewin

import "math"

// Report compares a limiter's view of traffic with the exact sliding log.
type Report struct {
	Requests     int
	WrongAllowed int     // exact rate over the limit, estimate not
	WrongLimited int     // estimate over the limit, exact rate not
	MeanError    float64 // mean of |estimate - exact| / exact, over all requests
	Sources      int
	Over         int // sources whose exact rate went over the limit at some point
	Missed       int // of those, never caught by the estimate (false negatives)
	Accused      int // caught by the estimate but never over (false positives)
	// WorstMissed is the highest exact rate, as a multiple of the limit,
	// reached by a source the estimate never caught.
	WorstMissed float64
}

// Wrong is the share of requests the estimate judged differently from the log.
func (r Report) Wrong() float64 {
	if r.Requests == 0 {
		return 0
	}
	return float64(r.WrongAllowed+r.WrongLimited) / float64(r.Requests)
}

type windowPair struct {
	index     int64
	prev, cur int64
}

// Evaluate replays reqs through the sliding window estimate and the exact log
// and compares, at every request, whether each says the client is over limit.
// Counting here is immediate; EvaluatePoP adds the delay of the real path.
func Evaluate(reqs []Request, rule Rule) Report {
	log := NewLog(rule.Window)
	pairs := map[string]*windowPair{}
	peak := map[string]float64{}
	caught := map[string]bool{}
	var r Report
	var errSum float64
	for _, q := range reqs {
		i, elapsed := Bucket(q.At, rule.Window)
		p := pairs[q.Client]
		if p == nil {
			p = &windowPair{index: i}
			pairs[q.Client] = p
		}
		switch {
		case i == p.index+1:
			p.prev, p.cur = p.cur, 0
		case i > p.index+1:
			p.prev, p.cur = 0, 0
		}
		p.index = i
		p.cur++
		log.Add(q.Client, q.At)

		exact := float64(log.Rate(q.Client, q.At))
		est := Estimate(p.prev, p.cur, elapsed, rule.Window)
		errSum += math.Abs(est-exact) / exact
		limit := float64(rule.Limit)
		switch {
		case exact > limit && est <= limit:
			r.WrongAllowed++
		case est > limit && exact <= limit:
			r.WrongLimited++
		}
		peak[q.Client] = max(peak[q.Client], exact/limit)
		if est > limit {
			caught[q.Client] = true
		}
	}
	r.Requests = len(reqs)
	if r.Requests > 0 {
		r.MeanError = errSum / float64(r.Requests)
	}
	r.Sources = len(peak)
	for c, p := range peak {
		switch {
		case p > 1 && !caught[c]:
			r.Over++
			r.Missed++
			r.WorstMissed = max(r.WorstMissed, p)
		case p > 1:
			r.Over++
		case caught[c]:
			r.Accused++
		}
	}
	return r
}
