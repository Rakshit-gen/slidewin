package slidewin

import (
	"fmt"
	"time"
)

// PoP is a data center: several edge servers, each also hosting one shard of
// the shared counter store. A client's connections can land on any server, so
// no single server sees all of its traffic.
type PoP struct {
	Store   *Cluster
	Servers []*Server
}

// NewPoP builds a PoP of n servers enforcing rule.
func NewPoP(n int, rule Rule, now func() time.Time) *PoP {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("edge%02d", i+1)
	}
	p := &PoP{Store: NewCluster(now, names...)}
	for range names {
		p.Servers = append(p.Servers, NewServer(NewLimiter(rule, p.Store, now)))
	}
	return p
}

// Allow sends a request from client to server i (taken modulo the PoP size,
// so callers can pass a connection number).
func (p *PoP) Allow(i int, client string) bool {
	return p.Servers[i%len(p.Servers)].Allow(client)
}

// Flush runs the counting queue on every server.
func (p *PoP) Flush() {
	for _, s := range p.Servers {
		s.Flush()
	}
}
