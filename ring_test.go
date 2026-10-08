package slidewin

import (
	"fmt"
	"testing"
)

func TestRingSpreadsKeys(t *testing.T) {
	r := NewRing("s1", "s2", "s3", "s4")
	count := map[string]int{}
	const n = 40000
	for i := 0; i < n; i++ {
		count[r.Lookup(fmt.Sprintf("client-%d", i))]++
	}
	for s, c := range count {
		if c < n/4*7/10 || c > n/4*13/10 {
			t.Errorf("%s got %d of %d keys", s, c, n)
		}
	}
}

func TestRingEmpty(t *testing.T) {
	if got := NewRing().Lookup("x"); got != "" {
		t.Fatalf("got %q", got)
	}
}
