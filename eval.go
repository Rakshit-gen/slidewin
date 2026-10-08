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

// Evaluator replays requests through the sliding window estimate and the
// exact log and compares, at every request, whether each says the client is
// over the limit. Counting here is immediate; the PoP simulation adds the
// delay of the real path.
type Evaluator struct {
	rule   Rule
	log    *Log
	pairs  map[string]*windowPair
	peak   map[string]float64 // highest exact rate seen, as a multiple of the limit
	caught map[string]bool
	r      Report
	errSum float64
}

// NewEvaluator starts an empty comparison for rule.
func NewEvaluator(rule Rule) *Evaluator {
	return &Evaluator{
		rule:   rule,
		log:    NewLog(rule.Window),
		pairs:  map[string]*windowPair{},
		peak:   map[string]float64{},
		caught: map[string]bool{},
	}
}

// Add feeds the next request. Requests must arrive in time order.
func (e *Evaluator) Add(q Request) {
	i, elapsed := Bucket(q.At, e.rule.Window)
	p := e.pairs[q.Client]
	if p == nil {
		p = &windowPair{index: i}
		e.pairs[q.Client] = p
	}
	switch {
	case i == p.index+1:
		p.prev, p.cur = p.cur, 0
	case i > p.index+1:
		p.prev, p.cur = 0, 0
	}
	p.index = i
	p.cur++
	e.log.Add(q.Client, q.At)

	exact := float64(e.log.Rate(q.Client, q.At))
	est := Estimate(p.prev, p.cur, elapsed, e.rule.Window)
	e.errSum += math.Abs(est-exact) / exact
	limit := float64(e.rule.Limit)
	switch {
	case exact > limit && est <= limit:
		e.r.WrongAllowed++
	case est > limit && exact <= limit:
		e.r.WrongLimited++
	}
	e.peak[q.Client] = max(e.peak[q.Client], exact/limit)
	if est > limit {
		e.caught[q.Client] = true
	}
	e.r.Requests++
}

// Report summarises everything added so far.
func (e *Evaluator) Report() Report {
	r := e.r
	if r.Requests > 0 {
		r.MeanError = e.errSum / float64(r.Requests)
	}
	r.Sources = len(e.peak)
	for c, p := range e.peak {
		switch {
		case p > 1 && !e.caught[c]:
			r.Over++
			r.Missed++
			r.WorstMissed = max(r.WorstMissed, p)
		case p > 1:
			r.Over++
		case e.caught[c]:
			r.Accused++
		}
	}
	return r
}

// Evaluate runs a whole slice of requests through an Evaluator.
func Evaluate(reqs []Request, rule Rule) Report {
	e := NewEvaluator(rule)
	for _, q := range reqs {
		e.Add(q)
	}
	return e.Report()
}
