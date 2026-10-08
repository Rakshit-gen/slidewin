package slidewin

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"time"
)

// Request is one synthetic request: who sent it, when, and which server its
// connection landed on.
type Request struct {
	At     time.Time
	Client string
	Conn   int
}

// Shape describes how one client's request rate changes over a run.
type Shape int

const (
	Steady Shape = iota // a constant rate
	Bursty              // on for a while, then off
	Ramp                // rising from zero to twice the given rate
)

func (s Shape) String() string {
	return [...]string{"steady", "bursty", "ramp"}[s]
}

// Source is one client. PerWindow is its average rate in requests per rule
// window (twice that while on, for Bursty, so the average holds).
type Source struct {
	Client    string
	Shape     Shape
	PerWindow float64
}

// rateAt is the source's instantaneous rate, in requests per window, at
// fraction f of the run.
func (s Source) rateAt(f float64, rng *rand.Rand, on *bool) float64 {
	switch s.Shape {
	case Bursty:
		if rng.Float64() < 0.002 {
			*on = !*on
		}
		if *on {
			return 2 * s.PerWindow
		}
		return 0
	case Ramp:
		return 2 * s.PerWindow * f
	}
	return s.PerWindow
}

// Stream calls fn with every request the sources send over length, in time
// order. It works one small step at a time, so memory does not grow with the
// length of the run. Arrivals are Poisson; servers is how many edge servers
// connections spread over.
func Stream(sources []Source, start time.Time, length, window time.Duration, servers int, seed uint64, fn func(Request)) {
	rng := rand.New(rand.NewPCG(seed, 0))
	step := window / 100
	on := make([]bool, len(sources))
	for i := range on {
		on[i] = rng.IntN(2) == 0
	}
	var batch []Request
	for t := time.Duration(0); t < length; t += step {
		batch = batch[:0]
		f := float64(t) / float64(length)
		for i, s := range sources {
			r := s.rateAt(f, rng, &on[i]) / 100
			for range poisson(rng, r) {
				at := t + time.Duration(rng.Float64()*float64(step))
				batch = append(batch, Request{At: start.Add(at), Client: s.Client, Conn: rng.IntN(servers)})
			}
		}
		slices.SortFunc(batch, func(a, b Request) int { return a.At.Compare(b.At) })
		for _, q := range batch {
			fn(q)
		}
	}
}

// Generate collects Stream's requests into a slice.
func Generate(sources []Source, start time.Time, length, window time.Duration, servers int, seed uint64) []Request {
	var out []Request
	Stream(sources, start, length, window, servers, seed, func(q Request) { out = append(out, q) })
	return out
}

func poisson(rng *rand.Rand, mean float64) int {
	// Knuth's method; the means here are small.
	l, k, p := math.Exp(-mean), 0, 1.0
	for {
		p *= rng.Float64()
		if p <= l {
			return k
		}
		k++
	}
}

// Mix is a population of clients around a limit. A share near of them sends
// between 0.7 and 1.3 times the limit, where the estimate and the exact count
// disagree most; 5% send 2 to 10 times the limit; the rest stay under half.
func Mix(n int, limit int64, near float64, seed uint64) []Source {
	rng := rand.New(rand.NewPCG(seed, 1))
	out := make([]Source, n)
	for i := range out {
		var per float64
		switch u := rng.Float64(); {
		case u < 0.95-near:
			per = rng.Float64() * 0.5
		case u < 0.95:
			per = 0.7 + rng.Float64()*0.6
		default:
			per = 2 + rng.Float64()*8
		}
		out[i] = Source{
			Client:    fmt.Sprintf("198.51.%d.%d", i/256%256, i%256),
			Shape:     Shape(rng.IntN(3)),
			PerWindow: per * float64(limit),
		}
	}
	return out
}
