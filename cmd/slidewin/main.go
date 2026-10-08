// Command slidewin replays synthetic traffic through the sliding window rate
// limiter and reports how its decisions compare with an exact count.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Rakshit-gen/slidewin"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "eval":
		eval(os.Args[2:])
	case "sim":
		sim(os.Args[2:])
	case "serve":
		serve(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: slidewin eval|sim|serve [flags]")
	os.Exit(2)
}

func eval(args []string) {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	clients := fs.Int("clients", 2000, "number of synthetic clients")
	minutes := fs.Int("minutes", 30, "length of the run in minutes")
	limit := fs.Int64("limit", 50, "requests allowed per window")
	window := fs.Duration("window", time.Minute, "rule window")
	seed := fs.Uint64("seed", 1, "random seed")
	near := fs.Float64("near", 0.15, "share of clients sending 0.7 to 1.3 times the limit")
	byShape := fs.Bool("by-shape", false, "also report steady, bursty and ramping clients separately")
	fs.Parse(args)

	rule := slidewin.Rule{Limit: *limit, Window: *window}
	src := slidewin.Mix(*clients, *limit, *near, *seed)
	length := time.Duration(*minutes) * time.Minute
	if *byShape {
		defer func() {
			fmt.Printf("\n%-8s %9s %8s %6s %6s %8s\n", "shape", "requests", "wrong", "error", "missed", "accused")
			for sh := slidewin.Steady; sh <= slidewin.Ramp; sh++ {
				var part []slidewin.Source
				for _, s := range src {
					if s.Shape == sh {
						part = append(part, s)
					}
				}
				e := slidewin.NewEvaluator(rule)
				slidewin.Stream(part, time.Unix(0, 0), length, *window, 1, *seed, e.Add)
				r := e.Report()
				fmt.Printf("%-8v %9d %7.3f%% %5.1f%% %6d %8d\n", sh, r.Requests, 100*r.Wrong(), 100*r.MeanError, r.Missed, r.Accused)
			}
		}()
	}
	start := time.Now()
	e := slidewin.NewEvaluator(rule)
	slidewin.Stream(src, time.Unix(0, 0), length, *window, 1, *seed, e.Add)
	r := e.Report()
	fmt.Printf("%d requests from %d sources, limit %d per %v (%.1fs)\n",
		r.Requests, r.Sources, rule.Limit, rule.Window, time.Since(start).Seconds())
	fmt.Printf("judged differently from the exact log: %.3f%% (%d wrongly allowed, %d wrongly limited)\n",
		100*r.Wrong(), r.WrongAllowed, r.WrongLimited)
	fmt.Printf("mean difference between estimate and exact rate: %.1f%%\n", 100*r.MeanError)
	fmt.Printf("sources over the limit: %d, never caught: %d", r.Over, r.Missed)
	if r.Missed > 0 {
		fmt.Printf(" (worst peaked at %.2fx the limit)", r.WorstMissed)
	}
	fmt.Printf("\nsources caught without ever going over: %d\n", r.Accused)
}

func sim(args []string) {
	fs := flag.NewFlagSet("sim", flag.ExitOnError)
	clients := fs.Int("clients", 2000, "number of synthetic clients")
	minutes := fs.Int("minutes", 30, "length of the run in minutes")
	limit := fs.Int64("limit", 50, "requests allowed per window")
	window := fs.Duration("window", time.Minute, "rule window")
	servers := fs.Int("servers", 16, "edge servers in the PoP")
	seed := fs.Uint64("seed", 1, "random seed")
	near := fs.Float64("near", 0.15, "share of clients sending 0.7 to 1.3 times the limit")
	fs.Parse(args)

	rule := slidewin.Rule{Limit: *limit, Window: *window}
	src := slidewin.Mix(*clients, *limit, *near, *seed)
	feed := func(fn func(slidewin.Request)) {
		slidewin.Stream(src, time.Unix(0, 0), time.Duration(*minutes)*time.Minute, *window, *servers, *seed, fn)
	}
	fmt.Printf("%d clients, %d servers, limit %d per %v\n\n", *clients, *servers, rule.Limit, rule.Window)
	fmt.Printf("%-10s %-6s %10s %8s %8s %11s %13s\n", "lag", "cache", "requests", "blocked", "leaked", "collateral", "trips/request")
	for _, lag := range []time.Duration{0, 100 * time.Millisecond, time.Second, 10 * time.Second} {
		for _, noCache := range []bool{false, true} {
			r := slidewin.Simulate(slidewin.SimConfig{Rule: rule, Servers: *servers, Lag: lag, NoCache: noCache}, feed)
			cache := "on"
			if noCache {
				cache = "off"
			}
			fmt.Printf("%-10v %-6s %10d %7.2f%% %7.2f%% %10.3f%% %13.2f\n", lag, cache, r.Requests,
				pct(r.Blocked, r.Requests), pct(r.Leaked, r.Requests), pct(r.Collateral, r.Requests),
				float64(r.RoundTrips)/float64(r.Requests))
		}
	}
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "listen address")
	limit := fs.Int64("limit", 10, "requests allowed per window")
	window := fs.Duration("window", 10*time.Second, "rule window")
	flush := fs.Duration("flush", 50*time.Millisecond, "how often queued requests are counted")
	fs.Parse(args)

	rule := slidewin.Rule{Limit: *limit, Window: *window}
	srv := slidewin.NewServer(slidewin.NewLimiter(rule, slidewin.NewCluster(nil, "local"), nil))
	go srv.Run(context.Background(), *flush)
	hello := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	log.Printf("listening on %s, %d requests per %v per client", *addr, rule.Limit, rule.Window)
	log.Fatal(http.ListenAndServe(*addr, slidewin.Middleware(srv, nil, hello)))
}
