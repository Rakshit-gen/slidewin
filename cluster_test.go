package slidewin

import (
	"errors"
	"fmt"
	"testing"
	"time"
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
		if got, _ := c.GetMulti(a, b); len(got) != 2 {
			t.Fatalf("%s: counters split across shards: %v", ip, got)
		}
	}
	if c.RoundTrips() != 300 {
		t.Fatalf("round trips: %d", c.RoundTrips())
	}
}

func TestClusterEjectsDeadShardAndRetries(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)}
	cl := NewCluster(c.now, "s1", "s2", "s3")
	// Find a key that lives on s2.
	var key string
	for i := 0; ; i++ {
		key = fmt.Sprintf("k%d", i)
		if cl.ring.Lookup(key) == "s2" {
			break
		}
	}
	cl.Node("s2").SetDown(true)
	for i := 0; i < ServerFailureLimit; i++ {
		if err := cl.Set(key, 1, 0); !errors.Is(err, ErrDown) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if got := cl.Ejected(); len(got) != 1 || got[0] != "s2" {
		t.Fatalf("ejected: %v", got)
	}
	if err := cl.Set(key, 1, 0); err != nil {
		t.Fatalf("after ejection the key should move: %v", err)
	}
	cl.Node("s2").SetDown(false)
	c.advance(ServerRetryTimeout)
	cl.Get(key)
	if len(cl.Ejected()) != 0 || cl.ring.Lookup(key) != "s2" {
		t.Fatal("s2 not back on the ring after the retry timeout")
	}
}
