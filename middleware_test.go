package slidewin

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiddleware(t *testing.T) {
	c := &clock{t: time.Unix(600, 0)}
	s := NewServer(NewLimiter(Rule{Limit: 2, Window: time.Minute}, NewCluster(c.now, "a"), c.now))
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("hi")) })
	h := Middleware(s, nil, ok)
	get := func(addr string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for i := 0; i < 3; i++ {
		if rec := get("192.0.2.1:5000"); rec.Code != 200 {
			t.Fatalf("request %d: %d", i, rec.Code)
		}
	}
	s.Flush()
	c.advance(500 * time.Millisecond)
	rec := get("192.0.2.1:6000") // a new connection, same client
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "60" {
		t.Fatalf("got %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if rec := get("192.0.2.2:5000"); rec.Code != 200 {
		t.Fatalf("other client: %d", rec.Code)
	}
}

func TestClientIP(t *testing.T) {
	for addr, want := range map[string]string{
		"192.0.2.1:80":      "192.0.2.1",
		"[2001:db8::1]:443": "2001:db8::1",
		"no-port":           "no-port",
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = addr
		if got := ClientIP(r); got != want {
			t.Errorf("%s: got %s", addr, got)
		}
	}
}
