// Command slidewin replays synthetic traffic through the sliding window rate
// limiter and reports how its decisions compare with an exact count.
package main

import (
	"flag"
	"fmt"
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
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: slidewin eval [flags]")
	os.Exit(2)
}

func eval(args []string) {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	clients := fs.Int("clients", 2000, "number of synthetic clients")
	minutes := fs.Int("minutes", 30, "length of the run in minutes")
	limit := fs.Int64("limit", 50, "requests allowed per window")
	window := fs.Duration("window", time.Minute, "rule window")
	seed := fs.Uint64("seed", 1, "random seed")
	fs.Parse(args)

	rule := slidewin.Rule{Limit: *limit, Window: *window}
	src := slidewin.Mix(*clients, *limit, *seed)
	start := time.Now()
	e := slidewin.NewEvaluator(rule)
	slidewin.Stream(src, time.Unix(0, 0), time.Duration(*minutes)*time.Minute, *window, 1, *seed, e.Add)
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
