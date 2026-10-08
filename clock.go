package slidewin

import "time"

// clock is a manual clock, so simulations can replay an hour of traffic in
// seconds and every component reads the same simulated time.
type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }
