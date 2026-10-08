package slidewin

import "time"

// Log is the exact answer: every request's timestamp, per client. It is what
// the sliding window approximates, and too costly to run at the edge, since
// memory grows with the limit and checking means scanning the log.
type Log struct {
	window time.Duration
	times  map[string][]time.Time
}

// NewLog returns an exact sliding log over window.
func NewLog(window time.Duration) *Log {
	return &Log{window: window, times: map[string][]time.Time{}}
}

// Add records a request from client at t. Times must not go backwards.
func (l *Log) Add(client string, t time.Time) {
	l.times[client] = append(l.trim(client, t), t)
}

// Rate is the number of requests from client in the window ending at t.
func (l *Log) Rate(client string, t time.Time) int {
	return len(l.trim(client, t))
}

func (l *Log) trim(client string, t time.Time) []time.Time {
	ts := l.times[client]
	cut := t.Add(-l.window)
	i := 0
	for i < len(ts) && !ts[i].After(cut) {
		i++
	}
	ts = ts[i:]
	l.times[client] = ts
	return ts
}
