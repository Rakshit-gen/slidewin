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

func TestBucket(t *testing.T) {
	now := time.Unix(125, 0) // 2 minutes and 5 seconds after the epoch
	i, e := Bucket(now, time.Minute)
	if i != 2 || e != 5*time.Second {
		t.Fatalf("got %d, %v", i, e)
	}
	j, _ := Bucket(now.Add(55*time.Second), time.Minute)
	if j != 3 {
		t.Fatalf("next window: got %d", j)
	}
}
