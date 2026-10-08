package slidewin

import (
	"testing"
	"time"
)

func TestSimulateLagLetsMoreThrough(t *testing.T) {
	rule := Rule{Limit: 50, Window: time.Minute}
	src := []Source{{Client: "attacker", Shape: Steady, PerWindow: 5000}}
	feed := func(fn func(Request)) {
		Stream(src, time.Unix(0, 0), 3*time.Minute, time.Minute, 8, 1, fn)
	}
	fast := Simulate(SimConfig{Rule: rule, Servers: 8, Lag: 0}, feed)
	slow := Simulate(SimConfig{Rule: rule, Servers: 8, Lag: 5 * time.Second}, feed)
	if fast.Leaked >= slow.Leaked {
		t.Fatalf("leaked %d with no lag, %d with 5 s lag", fast.Leaked, slow.Leaked)
	}
	if fast.Blocked < fast.Requests*9/10 {
		t.Fatalf("blocked only %d of %d", fast.Blocked, fast.Requests)
	}
}

func TestSimulateCacheSavesRoundTrips(t *testing.T) {
	rule := Rule{Limit: 50, Window: time.Minute}
	src := []Source{{Client: "attacker", Shape: Steady, PerWindow: 5000}}
	feed := func(fn func(Request)) {
		Stream(src, time.Unix(0, 0), 3*time.Minute, time.Minute, 8, 1, fn)
	}
	cached := Simulate(SimConfig{Rule: rule, Servers: 8, Lag: time.Second}, feed)
	uncached := Simulate(SimConfig{Rule: rule, Servers: 8, Lag: time.Second, NoCache: true}, feed)
	if cached.RoundTrips*10 > uncached.RoundTrips {
		t.Fatalf("round trips: %d cached, %d uncached", cached.RoundTrips, uncached.RoundTrips)
	}
	if cached.Blocked != uncached.Blocked {
		t.Fatalf("cache changed decisions: %d vs %d blocked", cached.Blocked, uncached.Blocked)
	}
}
