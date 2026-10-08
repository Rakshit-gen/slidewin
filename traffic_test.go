package slidewin

import (
	"testing"
	"time"
)

func TestGenerateHitsAverageRate(t *testing.T) {
	src := []Source{{Client: "a", Shape: Steady, PerWindow: 100}}
	reqs := Generate(src, time.Unix(0, 0), 50*time.Minute, time.Minute, 4, 1)
	if n := len(reqs); n < 4700 || n > 5300 {
		t.Fatalf("%d requests, want about 5000", n)
	}
	for i := 1; i < len(reqs); i++ {
		if reqs[i].At.Before(reqs[i-1].At) {
			t.Fatal("requests out of order")
		}
	}
}

func TestGenerateIsRepeatable(t *testing.T) {
	src := Mix(20, 50, 7)
	a := Generate(src, time.Unix(0, 0), 5*time.Minute, time.Minute, 4, 3)
	b := Generate(src, time.Unix(0, 0), 5*time.Minute, time.Minute, 4, 3)
	if len(a) != len(b) || len(a) == 0 || a[len(a)-1] != b[len(b)-1] {
		t.Fatal("same seed gave different traffic")
	}
}
