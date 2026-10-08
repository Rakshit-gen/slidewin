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

func TestRingResizeMovesFewKeys(t *testing.T) {
	r := NewRing("s1", "s2", "s3", "s4")
	const n = 20000
	before := make([]string, n)
	for i := range before {
		before[i] = r.Lookup(fmt.Sprintf("client-%d", i))
	}
	r.Add("s5")
	moved := 0
	for i := range before {
		got := r.Lookup(fmt.Sprintf("client-%d", i))
		if got != before[i] {
			moved++
			if got != "s5" {
				t.Fatalf("key moved between old servers: %s -> %s", before[i], got)
			}
		}
	}
	// About a fifth should move to the new server.
	if moved < n/5*7/10 || moved > n/5*13/10 {
		t.Fatalf("%d of %d keys moved", moved, n)
	}
	r.Remove("s5")
	for i := range before {
		if got := r.Lookup(fmt.Sprintf("client-%d", i)); got != before[i] {
			t.Fatalf("after removing s5, key %d is on %s, was %s", i, got, before[i])
		}
	}
}

func TestRingSpreadsIPAddresses(t *testing.T) {
	var names []string
	for i := 1; i <= 16; i++ {
		names = append(names, fmt.Sprintf("edge%02d", i))
	}
	r := NewRing(names...)
	count := map[string]int{}
	const n = 16000
	for i := 0; i < n; i++ {
		count[r.Lookup(fmt.Sprintf("198.51.%d.%d", i/256%256, i%256))]++
	}
	for _, s := range names {
		if c := count[s]; c < n/16*6/10 || c > n/16*14/10 {
			t.Errorf("%s got %d of %d keys", s, c, n)
		}
	}
}
