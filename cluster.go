package slidewin

import (
	"strings"
	"sync/atomic"
	"time"
)

// Cluster is the PoP-wide counter store: one Node per server, with keys spread
// over them by a consistent hash ring, the job Twemproxy does in front of
// memcached.
type Cluster struct {
	ring  *Ring
	nodes map[string]*Node
	trips atomic.Int64
}

// NewCluster creates one shard per server name.
func NewCluster(now func() time.Time, servers ...string) *Cluster {
	c := &Cluster{ring: NewRing(servers...), nodes: map[string]*Node{}}
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

func (c *Cluster) node(key string) *Node {
	c.trips.Add(1)
	return c.nodes[c.ring.Lookup(shardKey(key))]
}

// Get, Set and Incr go to the shard that owns key.
func (c *Cluster) Get(key string) (int64, error) { return c.node(key).Get(key) }

func (c *Cluster) Set(key string, val int64, ttl time.Duration) error {
	return c.node(key).Set(key, val, ttl)
}

func (c *Cluster) Incr(key string, delta int64) (int64, error) { return c.node(key).Incr(key, delta) }

// GetMulti fetches keys that share a hash tag in one round trip.
func (c *Cluster) GetMulti(keys ...string) (map[string]int64, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	return c.node(keys[0]).GetMulti(keys...)
}

// RoundTrips is how many requests have been sent to the shards so far.
func (c *Cluster) RoundTrips() int64 { return c.trips.Load() }

// Node returns the shard on the named server.
func (c *Cluster) Node(server string) *Node { return c.nodes[server] }
