package slidewin

import (
	"testing"
	"time"
)

func TestEvaluateSteadyTrafficIsAccurate(t *testing.T) {
	rule := Rule{Limit: 100, Window: time.Minute}
	src := []Source{
		{Client: "low", Shape: Steady, PerWindow: 30},
		{Client: "high", Shape: Steady, PerWindow: 300},
	}
	reqs := Generate(src, time.Unix(0, 0), 30*time.Minute, time.Minute, 1, 1)
	r := Evaluate(reqs, rule)
	if r.Over != 1 || r.Missed != 0 || r.Accused != 0 {
		t.Fatalf("%+v", r)
	}
	if r.MeanError > 0.1 {
		t.Fatalf("mean error %.3f on steady traffic", r.MeanError)
	}
}

func TestEvaluateCountsDisagreements(t *testing.T) {
	// 100 requests in the last second of a window, nothing after. Ten seconds
	// into the next window the log still holds all 100 but they all sit in the
	// previous window, which the estimate weighs at 50/60.
	rule := Rule{Limit: 90, Window: time.Minute}
	var reqs []Request
	t0 := time.Unix(659, 0)
	for i := 0; i < 100; i++ {
		reqs = append(reqs, Request{At: t0.Add(time.Duration(i) * 10 * time.Millisecond), Client: "x"})
	}
	reqs = append(reqs, Request{At: time.Unix(670, 0), Client: "x"})
	r := Evaluate(reqs, rule)
	if r.WrongAllowed != 1 {
		t.Fatalf("%+v", r)
	}
}
