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
