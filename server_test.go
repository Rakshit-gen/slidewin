package slidewin

import (
	"context"
	"testing"
	"time"
)

func TestServerBlocksAfterFlush(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)}
	l := NewLimiter(Rule{Limit: 3, Window: time.Minute}, NewCluster(c.now, "a"), c.now)
	s := NewServer(l)
	for i := 0; i < 5; i++ {
		if !s.Allow("x") {
			t.Fatalf("request %d blocked before counting ran", i)
		}
	}
	s.Flush()
	if s.Allow("x") {
		t.Fatal("still allowed after counting 5 against a limit of 3")
	}
	if !s.Allow("y") {
		t.Fatal("other client blocked")
	}
}

func TestServerCachesMitigation(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)}
	store := NewCluster(c.now, "a")
	l := NewLimiter(Rule{Limit: 1, Window: time.Minute}, store, c.now)
	s := NewServer(l)
	s.Allow("x")
	s.Allow("x")
	s.Flush()
	s.Allow("x") // reads the flag once
	before := store.RoundTrips()
	for i := 0; i < 1000; i++ {
		if s.Allow("x") {
			t.Fatal("allowed during mitigation")
		}
	}
	if n := store.RoundTrips() - before; n != 0 {
		t.Fatalf("%d store round trips while mitigation was cached", n)
	}
	c.advance(time.Minute)
	if !s.Allow("x") {
		t.Fatal("still blocked after the mitigation ended")
	}
}

func TestServerRunFlushesInBackground(t *testing.T) {
	l := NewLimiter(Rule{Limit: 2, Window: time.Hour}, NewCluster(nil, "a"), nil)
	s := NewServer(l)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx, time.Millisecond); close(done) }()
	for i := 0; i < 3; i++ {
		s.Allow("x")
	}
	deadline := time.Now().Add(2 * time.Second)
	for s.Allow("x") {
		if time.Now().After(deadline) {
			t.Fatal("background flush never mitigated the client")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
}
