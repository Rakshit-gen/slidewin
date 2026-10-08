// Package slidewin is a sliding window rate limiter built the way Cloudflare
// described theirs in 2017: two counters per client, a memcached-style store
// that only offers GET, SET and INCR, increments sent off the request path, and
// mitigation decisions cached in each server's memory.
package slidewin

import "time"

// Estimate returns the approximate number of requests seen in the last window.
// It assumes the previous window's requests were spread evenly, so the part of
// the previous window still inside the sliding window counts in proportion:
//
//	rate = prev * (window - elapsed) / window + cur
func Estimate(prev, cur int64, elapsed, window time.Duration) float64 {
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > window {
		elapsed = window
	}
	weight := float64(window-elapsed) / float64(window)
	return float64(prev)*weight + float64(cur)
}
