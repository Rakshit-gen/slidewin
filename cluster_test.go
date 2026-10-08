package slidewin

import (
	"fmt"
	"testing"
)

func TestShardKey(t *testing.T) {
	for in, want := range map[string]string{
		"rl:{1.2.3.4}:17": "1.2.3.4",
		"plain":           "plain",
		"open{":           "open{",
		"{}empty":         "{}empty",
	} {
		if got := shardKey(in); got != want {
			t.Errorf("shardKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClusterKeepsTaggedKeysTogether(t *testing.T) {
	c := NewCluster(nil, "s1", "s2", "s3")
	for i := 0; i < 100; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i)
		a, b := fmt.Sprintf("rl:{%s}:1", ip), fmt.Sprintf("rl:{%s}:2", ip)
		c.Set(a, 1, 0)
		c.Set(b, 2, 0)
		if got := c.GetMulti(a, b); len(got) != 2 {
			t.Fatalf("%s: counters split across shards: %v", ip, got)
		}
	}
	if c.RoundTrips() != 300 {
		t.Fatalf("round trips: %d", c.RoundTrips())
	}
}
