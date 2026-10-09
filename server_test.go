package slidewin

import (
	"context"
	"fmt"
	"sync"
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

func TestCheckSaysWhenMitigationEnds(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)}
	l := NewLimiter(Rule{Limit: 1, Window: time.Minute, Timeout: 5 * time.Minute}, NewCluster(c.now, "a"), c.now)
	s := NewServer(l)
	s.Allow("x")
	s.Allow("x")
	s.Flush()
	c.advance(time.Minute)
	if wait, blocked := s.Check("x"); !blocked || wait != 4*time.Minute {
		t.Fatalf("got %v %v", wait, blocked)
	}
	c.advance(time.Minute) // now from the in-memory copy
	if wait, blocked := s.Check("x"); !blocked || wait != 3*time.Minute {
		t.Fatalf("cached: got %v %v", wait, blocked)
	}
}

func TestServerIsSafeForConcurrentUse(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)} // never advanced, so only read
	s := NewServer(NewLimiter(Rule{Limit: 50, Window: time.Minute}, NewCluster(c.now, "a", "b"), c.now))
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				s.Allow(fmt.Sprintf("c%d", (g*500+i)%50))
				if i%100 == 0 {
					s.Flush()
				}
			}
		}(g)
	}
	wg.Wait()
	s.Flush()
	// Each client sent 80. Racing first increments can lose a few counts,
	// never 30.
	for i := 0; i < 50; i++ {
		if s.Allow(fmt.Sprintf("c%d", i)) {
			t.Fatalf("c%d not mitigated", i)
		}
	}
}

func TestServerQueueIsBounded(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)}
	s := NewServer(NewLimiter(Rule{Limit: 1000, Window: time.Minute}, NewCluster(c.now, "a"), c.now))
	s.MaxPending = 10
	for i := 0; i < 25; i++ {
		s.Allow("x")
	}
	if s.Dropped.Load() != 15 {
		t.Fatalf("dropped %d", s.Dropped.Load())
	}
	s.Flush()
	if got := s.lim.Rate("x"); got != 10 {
		t.Fatalf("counted %v", got)
	}
}

func TestFlushCountsEachClientOnce(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)}
	store := NewCluster(c.now, "a")
	s := NewServer(NewLimiter(Rule{Limit: 1000, Window: time.Minute}, store, c.now))
	for i := 0; i < 500; i++ {
		s.Allow("x")
	}
	before := store.RoundTrips()
	s.Flush()
	// The INCR misses and falls back to SET, then Judge reads both windows.
	if n := store.RoundTrips() - before; n != 3 {
		t.Fatalf("%d round trips to count 500 requests from one client", n)
	}
	if r := s.lim.Rate("x"); r != 500 {
		t.Fatalf("rate %v, want 500", r)
	}
}

func TestFlushForgetsEndedMitigations(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)}
	s := NewServer(NewLimiter(Rule{Limit: 1, Window: time.Minute}, NewCluster(c.now, "a"), c.now))
	for i := 0; i < 100; i++ {
		ip := fmt.Sprint("10.0.0.", i)
		s.Allow(ip)
		s.Allow(ip)
	}
	s.Flush()
	for i := 0; i < 100; i++ {
		s.Allow(fmt.Sprint("10.0.0.", i)) // caches the mitigation
	}
	if len(s.blocked) != 100 {
		t.Fatalf("%d cached", len(s.blocked))
	}
	c.advance(time.Minute)
	s.Flush()
	if len(s.blocked) != 0 {
		t.Fatalf("%d ended mitigations still cached", len(s.blocked))
	}
}
