package slidewin

import (
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
