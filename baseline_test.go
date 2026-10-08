package slidewin

import (
	"testing"
	"time"
)

func TestLogCountsExactly(t *testing.T) {
	l := NewLog(time.Minute)
	t0 := time.Unix(0, 0)
	for i := 0; i < 10; i++ {
		l.Add("x", t0.Add(time.Duration(i)*10*time.Second))
	}
	// Requests at 0..90 s. The window ending at 95 s holds 40..90 s.
	if got := l.Rate("x", t0.Add(95*time.Second)); got != 6 {
		t.Fatalf("got %d, want 6", got)
	}
	if got := l.Rate("y", t0); got != 0 {
		t.Fatalf("unseen client: %d", got)
	}
}

func TestFixedLetsBoundaryBurstThrough(t *testing.T) {
	const limit = 10
	f := NewFixed(time.Minute)
	l := NewLog(time.Minute)
	boundary := time.Unix(600, 0)
	allowed := 0
	// 10 requests in the last second of one minute, 10 in the first of the next.
	for i := 0; i < 20; i++ {
		at := boundary.Add(time.Duration(i-10) * 100 * time.Millisecond)
		l.Add("x", at)
		if f.Add("x", at) <= limit {
			allowed++
		}
	}
	if allowed != 20 {
		t.Fatalf("fixed window allowed %d of 20", allowed)
	}
	if got := l.Rate("x", boundary.Add(time.Second)); got != 20 {
		t.Fatalf("true rate %d", got)
	}
	// The sliding estimate sees nearly all of it.
	if got := Estimate(10, 10, time.Second, time.Minute); got < 19 {
		t.Fatalf("sliding estimate %v", got)
	}
}
