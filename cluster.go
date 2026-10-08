package slidewin

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Ejection settings, named after the Twemproxy options they copy.
const (
	// ServerFailureLimit is how many failures in a row take a shard off the ring.
	ServerFailureLimit = 2
	// ServerRetryTimeout is how long an ejected shard stays off the ring
	// before it is tried again.
	ServerRetryTimeout = 30 * time.Second
)

// Cluster is the PoP-wide counter store: one Node per server, with keys spread
// over them by a consistent hash ring, the job Twemproxy does in front of
// memcached. Like Twemproxy with auto_eject_hosts, it takes a failing shard
// off the ring so its keys move to the others instead of failing for as long
// as it is down.
type Cluster struct {
	now   func() time.Time
	nodes map[string]*Node
	trips atomic.Int64

	mu       sync.Mutex
	ring     *Ring
	failures map[string]int
	ejected  map[string]time.Time // server -> when to try it again
}

// NewCluster creates one shard per server name.
func NewCluster(now func() time.Time, servers ...string) *Cluster {
	if now == nil {
		now = time.Now
	}
	c := &Cluster{
		now:      now,
		ring:     NewRing(servers...),
		nodes:    map[string]*Node{},
		failures: map[string]int{},
		ejected:  map[string]time.Time{},
	}
	for _, s := range servers {
		c.nodes[s] = NewNode(now)
	}
	return c
}

// shardKey returns the part of key that decides its shard. As with Twemproxy's
// hash_tag, only the text inside {braces} is hashed when present, so both of a
// client's counters live on the same shard and one GET can fetch them.
func shardKey(key string) string {
	if i := strings.IndexByte(key, '{'); i >= 0 {
		if j := strings.IndexByte(key[i+1:], '}'); j > 0 {
			return key[i+1 : i+1+j]
		}
	}
	return key
}

// do runs op against the shard that owns key and keeps the failure counts.
func (c *Cluster) do(key string, op func(*Node) error) error {
	c.trips.Add(1)
	c.mu.Lock()
	now := c.now()
	for s, retry := range c.ejected {
		if !now.Before(retry) {
			delete(c.ejected, s)
			c.ring.Add(s)
		}
	}
	server := c.ring.Lookup(shardKey(key))
	c.mu.Unlock()
	if server == "" {
		return ErrDown
	}

	err := op(c.nodes[server])

	c.mu.Lock()
	defer c.mu.Unlock()
	if !errors.Is(err, ErrDown) {
		c.failures[server] = 0
		return err
	}
	c.failures[server]++
	if _, out := c.ejected[server]; !out && c.failures[server] >= ServerFailureLimit {
		c.failures[server] = 0
		c.ejected[server] = now.Add(ServerRetryTimeout)
		c.ring.Remove(server)
	}
	return err
}

// Get, Set and Incr go to the shard that owns key.
func (c *Cluster) Get(key string) (v int64, err error) {
	err = c.do(key, func(n *Node) error { v, err = n.Get(key); return err })
	return v, err
}

func (c *Cluster) Set(key string, val int64, ttl time.Duration) error {
	return c.do(key, func(n *Node) error { return n.Set(key, val, ttl) })
}

func (c *Cluster) Incr(key string, delta int64) (v int64, err error) {
	err = c.do(key, func(n *Node) error { v, err = n.Incr(key, delta); return err })
	return v, err
}

// GetMulti fetches keys that share a hash tag in one round trip.
func (c *Cluster) GetMulti(keys ...string) (got map[string]int64, err error) {
	if len(keys) == 0 {
		return nil, nil
	}
	err = c.do(keys[0], func(n *Node) error { got, err = n.GetMulti(keys...); return err })
	return got, err
}

// RoundTrips is how many requests have been sent to the shards so far.
func (c *Cluster) RoundTrips() int64 { return c.trips.Load() }

// Node returns the shard on the named server.
func (c *Cluster) Node(server string) *Node { return c.nodes[server] }

// Ejected lists the servers currently off the ring.
func (c *Cluster) Ejected() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for s := range c.ejected {
		out = append(out, s)
	}
	return out
}
