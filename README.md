# slidewin

A sliding window rate limiter built the way Cloudflare described theirs in
[How we built rate limiting capable of scaling to millions of domains](https://blog.cloudflare.com/counting-things-a-lot-of-different-things/)
(2017), plus a simulator that measures how far its decisions drift from an exact count.

Not affiliated with Cloudflare. Everything here comes from that public post; the code and numbers are mine.

## The design in the post, and where it lives here

| From the post | Here |
| --- | --- |
| Two counters per client, previous and current window. `rate = prev * (window - elapsed) / window + cur`, so 42 last minute and 18 in the 15 s of this one gives 49.5 | `Estimate` in [window.go](window.go), with that exact example as a test |
| Counters in a memcached cluster inside each PoP, split over servers by Twemproxy with consistent hashing | `Node` ([node.go](node.go)) offers only GET, SET and INCR. `Cluster` ([cluster.go](cluster.go)) shards over a ketama ring ([ring.go](ring.go)) |
| Both counters fetched with one GET | Keys use a Twemproxy-style hash tag, `rl:name:{client}:window`, so a client's counters share a shard and `GetMulti` reads both in one round trip |
| No leaky bucket: it needs operations memcached can't do atomically, and CAS fails constantly under attack | INCR fails on a missing key, so the first request of a window falls back to SET. Two servers racing there can lose a count, which the limiter accepts |
| Increments run off the request path | `Server.Allow` only checks for a mitigation and queues the client. `Flush` (or `Run` in the background) counts and judges |
| Over the limit, a flag tells every server in the PoP to mitigate | `Limiter.Judge` stores `mit:name:{client}` holding the end time, with a matching TTL |
| The end time is known, so servers keep the mitigation in memory and stop querying | `Server` caches it until it ends. An attacker's requests then cost no store round trips |

Things the post doesn't cover that I had to decide: the limiter fails open when a shard is down
(less protection, never blocked customers), and the cluster ejects a shard after 2 failures in a row and
tries it again after 30 s, like Twemproxy's `auto_eject_hosts`.

## How close the estimate is

`slidewin eval` replays synthetic traffic through the estimate and through an exact sliding log of every
request, and compares them at each request. 2000 clients over 30 minutes, limit 50 per minute:

```
$ slidewin eval -by-shape
2025956 requests from 1995 sources, limit 50 per 1m0s (1.1s)
judged differently from the exact log: 1.198% (10995 wrongly allowed, 13281 wrongly limited)
mean difference between estimate and exact rate: 4.4%
sources over the limit: 660, never caught: 35 (worst peaked at 1.10x the limit)
sources caught without ever going over: 2

shape     requests    wrong  error missed  accused
steady      629266   1.631%   4.3%      3        0
bursty      756544   1.066%   4.2%     15        2
ramp        649087   0.851%   4.0%     14        1
```

The post reports 0.003% wrong decisions over 400 million real requests from 270,000 sources, a 6% average
difference, 3 sources missed (all under 15% over) and no false positives. My 1.2% is far higher, and the
reason is the traffic, not the algorithm: I put 15% of clients between 0.7 and 1.3 times the limit, which is
the only place the two methods can disagree, and real traffic is mostly nowhere near a limit. With
`-near 0` it falls to 0.6%. What does match is the shape of the result: the average difference sits in the
same few percent, the sources it misses peak at most 10 to 18% over the limit, and it almost never blocks a
client that stayed under.

The same eval shows why the fixed window loses: a test in [baseline_test.go](baseline_test.go) sends the
limit at the end of one window and again at the start of the next, and the fixed counter lets all of it
through at twice the limit.

## The whole PoP

`slidewin sim` runs the same traffic through 16 servers sharing one store, with connections landing on random
servers. "Leaked" is a request served that took the client over the limit, counting what was actually served.
"Collateral" is a request blocked while the client was sending at or under the limit:

```
$ slidewin sim
lag        cache    requests  blocked   leaked  collateral trips/request
0s         on        2025956   54.96%    1.09%      1.027%          1.42
0s         off       2025956   54.96%    1.09%      1.027%          1.93
100ms      on        2025956   54.94%    1.10%      1.033%          1.42
1s         on        2025956   54.69%    1.23%      1.068%          1.42
10s        on        2025956   52.62%    3.80%      2.246%          1.39
```

- Counting a second behind the requests costs very little; at 10 s behind, leaks go up 3.5 times.
- The in-memory copy of mitigations doesn't change a single decision and saves a quarter of all store round
  trips here. Under a real attack, where most requests come from mitigated clients, the share is larger.
- Most collateral is the mitigation timeout doing its job: a client that bursts over stays blocked for a
  window even after it slows down.
- `-kill 15m` takes one of the 16 shards down halfway. Its clients start counting again from zero on the
  shards that take over, and leaks go from 1.09% to 1.10%.

## Speed

Apple M-series, `go test -bench .`:

| | ns per request |
| --- | --- |
| `Allow`, client already mitigated (answered from memory) | 25 |
| `Allow`, client under the limit (one flag lookup, queue) | 590 |
| `Allow` plus counting, 1000 clients | 1380 |

The store here is in-process, so these numbers leave out the network trip to memcached that a real PoP pays.
That trip is exactly what the post keeps off the request path.

## Use it

```sh
go install github.com/Rakshit-gen/slidewin/cmd/slidewin@latest
slidewin eval            # estimate vs exact count
slidewin sim             # whole PoP: lag, cache, leaks
slidewin serve -limit 5  # HTTP server behind the middleware, try it with curl
```

In your own server:

```go
rule := slidewin.Rule{Name: "api", Limit: 600, Window: time.Minute}
srv := slidewin.NewServer(slidewin.NewLimiter(rule, slidewin.NewCluster(nil, "local"), nil))
go srv.Run(ctx, 50*time.Millisecond)
http.ListenAndServe(":8080", slidewin.Middleware(srv, nil, handler))
```

Blocked requests get 429 with `Retry-After` set to when the mitigation ends.

## Limits

- The store is in memory in one process. It models memcached's behaviour (operations, TTLs, misses, lost data
  on restart) but there is no wire protocol, so a PoP is a simulation, not a deployment.
- The traffic is synthetic: Poisson arrivals in steady, bursty and ramping shapes. It is meant to be hard on
  the estimate, not to look like real traffic.
- The counting queue holds at most 65536 requests per server between flushes. Past that, requests are served
  and not counted (`Server.Dropped` says how many).

## License

MIT
