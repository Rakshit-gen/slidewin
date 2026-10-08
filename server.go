package slidewin

import (
	"sync"
	"time"
)

// Server is one edge server in a PoP. On the request path it only checks the
// mitigation flag. Counting and judging are queued and done later, so a busy
// counter store never slows down a request.
type Server struct {
	lim *Limiter

	mu      sync.Mutex
	pending []string
	blocked map[string]time.Time // mitigations this server already knows about
}

// NewServer returns a server that uses lim for counting.
func NewServer(lim *Limiter) *Server { return &Server{lim: lim, blocked: map[string]time.Time{}} }

// Allow decides whether to serve a request from client.
// A mitigation's end time is known when it starts, so once this server has
// seen one it keeps it in memory and stops asking the store about that client
// until it ends. During an attack the attacker's requests cost no round trips.
func (s *Server) Allow(client string) bool {
	now := s.lim.now()
	s.mu.Lock()
	end, cached := s.blocked[client]
	if cached && now.Before(end) {
		s.mu.Unlock()
		return false
	}
	delete(s.blocked, client)
	s.mu.Unlock()

	if end, blocked := s.lim.Mitigated(client); blocked {
		s.mu.Lock()
		s.blocked[client] = end
		s.mu.Unlock()
		return false
	}
	s.mu.Lock()
	s.pending = append(s.pending, client)
	s.mu.Unlock()
	return true
}

// Flush counts every queued request and judges each client it touched. A real
// server runs this in the background; simulations call it at chosen moments
// to model how far counting lags behind requests.
func (s *Server) Flush() {
	s.mu.Lock()
	batch := s.pending
	s.pending = nil
	s.mu.Unlock()
	seen := map[string]bool{}
	for _, c := range batch {
		s.lim.Count(c)
		seen[c] = true
	}
	for c := range seen {
		s.lim.Judge(c)
	}
}
