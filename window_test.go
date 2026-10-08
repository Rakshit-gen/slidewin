package slidewin

import (
	"testing"
	"time"
)

func TestEstimateBlogExample(t *testing.T) {
	// 42 requests last minute, 18 so far in a minute that began 15 s ago.
	got := Estimate(42, 18, 15*time.Second, time.Minute)
	if got != 49.5 {
		t.Fatalf("got %v, want 49.5", got)
	}
}

func TestEstimateEdges(t *testing.T) {
	if got := Estimate(10, 0, 0, time.Minute); got != 10 {
		t.Fatalf("start of window: got %v, want 10", got)
	}
	if got := Estimate(10, 3, time.Minute, time.Minute); got != 3 {
		t.Fatalf("end of window: got %v, want 3", got)
	}
	if got := Estimate(10, 3, 2*time.Minute, time.Minute); got != 3 {
		t.Fatalf("past the window: got %v, want 3", got)
	}
}
