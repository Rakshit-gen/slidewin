package slidewin

import (
	"testing"
	"time"
)

func TestPoPSharesCountsAcrossServers(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)}
	p := NewPoP(4, Rule{Limit: 10, Window: time.Minute}, c.now)
	// 12 requests, 3 on each server: no server alone sees more than the limit.
	for i := 0; i < 12; i++ {
		p.Allow(i, "x")
	}
	p.Flush()
	for i := 0; i < 4; i++ {
		if p.Allow(i, "x") {
			t.Fatalf("server %d still allows x", i)
		}
	}
}
