package slidewin

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkEstimate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Estimate(42, int64(i), 15*time.Second, time.Minute)
	}
}

// A client under the limit: one flag lookup and a queued count.
func BenchmarkAllowUnderLimit(b *testing.B) {
	s := NewServer(NewLimiter(Rule{Limit: 1 << 40, Window: time.Minute}, NewCluster(nil, "a", "b", "c"), nil))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Allow("192.0.2.1")
		if i%1024 == 0 {
			s.Flush()
		}
	}
}

// An attacker already mitigated: answered from memory.
func BenchmarkAllowMitigated(b *testing.B) {
	c := &clock{t: time.Unix(600, 0)}
	s := NewServer(NewLimiter(Rule{Limit: 1, Window: time.Minute}, NewCluster(c.now, "a", "b", "c"), c.now))
	s.Allow("x")
	s.Allow("x")
	s.Flush()
	s.Allow("x")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Allow("x")
	}
}

func BenchmarkFlush(b *testing.B) {
	s := NewServer(NewLimiter(Rule{Limit: 50, Window: time.Minute}, NewCluster(nil, "a", "b", "c"), nil))
	clients := make([]string, 1000)
	for i := range clients {
		clients[i] = fmt.Sprintf("10.0.%d.%d", i/256, i%256)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Allow(clients[i%len(clients)])
		if i%1000 == 999 {
			s.Flush()
		}
	}
}
