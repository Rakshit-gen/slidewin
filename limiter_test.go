package slidewin

import (
	"testing"
	"time"
)

func TestLimiterReproducesBlogExample(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)} // the start of a minute
	l := NewLimiter(Rule{Limit: 50, Window: time.Minute}, NewCluster(c.now, "a", "b"), c.now)
	for i := 0; i < 42; i++ {
		l.Count("1.2.3.4")
	}
	c.advance(time.Minute)
	for i := 0; i < 18; i++ {
		l.Count("1.2.3.4")
	}
	c.advance(15 * time.Second)
	if got := l.Rate("1.2.3.4"); got != 49.5 {
		t.Fatalf("rate %v, want 49.5", got)
	}
	if got := l.Rate("5.6.7.8"); got != 0 {
		t.Fatalf("unseen client rate %v", got)
	}
}

func TestLimiterCountersExpire(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)}
	l := NewLimiter(Rule{Limit: 50, Window: time.Minute}, NewCluster(c.now, "a"), c.now)
	l.Count("x")
	c.advance(2 * time.Minute)
	if _, err := l.store.Get(counterKey("x", 10)); err == nil {
		t.Fatal("counter outlived two windows")
	}
}

func TestJudgeSetsFlagUntilTimeout(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)}
	l := NewLimiter(Rule{Limit: 5, Window: time.Minute, Timeout: 10 * time.Minute}, NewCluster(c.now, "a"), c.now)
	for i := 0; i < 5; i++ {
		l.Count("x")
	}
	if l.Judge("x") {
		t.Fatal("mitigated at the limit")
	}
	l.Count("x")
	if !l.Judge("x") {
		t.Fatal("not mitigated over the limit")
	}
	end, ok := l.Mitigated("x")
	if !ok || !end.Equal(c.t.Add(10*time.Minute)) {
		t.Fatalf("flag: %v %v", end, ok)
	}
	c.advance(10 * time.Minute)
	if _, ok := l.Mitigated("x"); ok {
		t.Fatal("flag outlived its timeout")
	}
}
