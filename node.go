package slidewin

import (
	"errors"
	"sync"
	"time"
)

// ErrMiss is returned when a key is absent or has expired, as memcached does.
var ErrMiss = errors.New("slidewin: key not found")

// Node is one memcached shard. Like the real thing it only offers GET, SET and
// INCR. INCR is atomic but fails on a missing key, so there is no single call
// that creates a counter and increments it.
type Node struct {
	mu    sync.Mutex
	items map[string]item
	now   func() time.Time
}

type item struct {
	val     int64
	expires time.Time // zero means never
}

// NewNode returns an empty shard that reads time from now (time.Now if nil).
func NewNode(now func() time.Time) *Node {
	if now == nil {
		now = time.Now
	}
	return &Node{items: map[string]item{}, now: now}
}

func (n *Node) live(key string) (item, bool) {
	it, ok := n.items[key]
	if ok && !it.expires.IsZero() && !n.now().Before(it.expires) {
		delete(n.items, key)
		return item{}, false
	}
	return it, ok
}

// Get returns the value stored under key.
func (n *Node) Get(key string) (int64, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	it, ok := n.live(key)
	if !ok {
		return 0, ErrMiss
	}
	return it.val, nil
}

// Set stores val under key for ttl (forever if ttl is 0).
func (n *Node) Set(key string, val int64, ttl time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()
	it := item{val: val}
	if ttl > 0 {
		it.expires = n.now().Add(ttl)
	}
	n.items[key] = it
}

// Incr adds delta to an existing counter and returns the new value. The expiry
// set by Set is kept.
func (n *Node) Incr(key string, delta int64) (int64, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	it, ok := n.live(key)
	if !ok {
		return 0, ErrMiss
	}
	it.val += delta
	n.items[key] = it
	return it.val, nil
}
