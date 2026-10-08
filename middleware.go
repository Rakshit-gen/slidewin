package slidewin

import (
	"math"
	"net"
	"net/http"
	"strconv"
)

// Middleware rate limits next by the key that key returns for each request
// (ClientIP if nil). Blocked requests get 429 with a Retry-After header in
// whole seconds, rounded up.
func Middleware(s *Server, key func(*http.Request) string, next http.Handler) http.Handler {
	if key == nil {
		key = ClientIP
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wait, blocked := s.Check(key(r)); blocked {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP is the request's remote address without the port.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
