// Command loadgen is a standalone traffic generator for the VOIS Speechmark
// telecom demo service. Given a base URL and API key, it loops creating plans
// and subscribers, ingesting usage, fetching invoices, and occasionally hitting
// scenario endpoints (ping, webhook register) so that metrics, traces, and logs
// populate immediately for the observability demo.
//
// It deliberately imports no internal/* packages: it exercises the real HTTP
// wire contract exactly as an external client would.
//
// Usage:
//
//	go run ./cmd/loadgen -url http://localhost:8080 -key vois-demo-secret-key -rps 20 -duration 60s
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// config holds the parsed command-line flags.
type config struct {
	baseURL  string
	apiKey   string
	rps      int
	duration time.Duration
	timeout  time.Duration
	seed     int64
	verbose  bool
}

func parseFlags(args []string) (config, error) {
	fs := flag.NewFlagSet("loadgen", flag.ContinueOnError)
	var c config
	fs.StringVar(&c.baseURL, "url", "http://localhost:8080", "base URL of the demo server")
	fs.StringVar(&c.apiKey, "key", "vois-demo-secret-key", "API key sent as Authorization: Bearer <key>")
	fs.IntVar(&c.rps, "rps", 10, "approximate target requests per second")
	fs.DurationVar(&c.duration, "duration", 30*time.Second, "how long to drive traffic (e.g. 30s, 5m)")
	fs.DurationVar(&c.timeout, "timeout", 10*time.Second, "per-request HTTP timeout")
	fs.Int64Var(&c.seed, "seed", time.Now().UnixNano(), "PRNG seed for reproducible traffic mixes")
	fs.BoolVar(&c.verbose, "v", false, "log every request line")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if c.rps <= 0 {
		return config{}, fmt.Errorf("-rps must be > 0, got %d", c.rps)
	}
	if c.duration <= 0 {
		return config{}, fmt.Errorf("-duration must be > 0, got %s", c.duration)
	}
	if c.baseURL == "" {
		return config{}, fmt.Errorf("-url must not be empty")
	}
	return c, nil
}

// stats accumulates outcome counts across all worker goroutines.
type stats struct {
	total    atomic.Int64
	ok       atomic.Int64 // 2xx
	client4xx atomic.Int64 // 4xx
	server   atomic.Int64 // 5xx
	errored  atomic.Int64 // transport error / non-decodable
}

func (s *stats) record(status int, err error) {
	s.total.Add(1)
	switch {
	case err != nil:
		s.errored.Add(1)
	case status >= 200 && status < 300:
		s.ok.Add(1)
	case status >= 400 && status < 500:
		s.client4xx.Add(1)
	default:
		s.server.Add(1)
	}
}

func (s *stats) String() string {
	return fmt.Sprintf("total=%d ok=%d 4xx=%d 5xx=%d errors=%d",
		s.total.Load(), s.ok.Load(), s.client4xx.Load(), s.server.Load(), s.errored.Load())
}

// httpClient is a thin HTTP wrapper that injects the bearer token and the base URL.
type httpClient struct {
	http    *http.Client
	baseURL string
	apiKey  string
	verbose bool
	stats   *stats
}

func newHTTPClient(c config, st *stats) *httpClient {
	return &httpClient{
		http:    &http.Client{Timeout: c.timeout},
		baseURL: c.baseURL,
		apiKey:  c.apiKey,
		verbose: c.verbose,
		stats:   st,
	}
}

// do issues a request to path (relative to baseURL). If body is non-nil it is
// JSON-encoded. If out is non-nil the response body is JSON-decoded into it.
// The /v1 auth header is always attached; unauth endpoints simply ignore it.
func (cl *httpClient) do(ctx context.Context, method, path string, body, out any) int {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			cl.stats.record(0, err)
			return 0
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, cl.baseURL+path, reader)
	if err != nil {
		cl.stats.record(0, err)
		return 0
	}
	req.Header.Set("Authorization", "Bearer "+cl.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := cl.http.Do(req)
	if err != nil {
		cl.stats.record(0, err)
		if cl.verbose {
			log.Printf("%-6s %-45s ERR %v", method, path, err)
		}
		return 0
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Ignore decode errors: read traffic only needs the status to count.
		_ = json.NewDecoder(resp.Body).Decode(out)
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	cl.stats.record(resp.StatusCode, nil)
	if cl.verbose {
		log.Printf("%-6s %-45s %d", method, path, resp.StatusCode)
	}
	return resp.StatusCode
}

// idHolder captures the {"id": "..."} field from create responses.
type idHolder struct {
	ID string `json:"id"`
}

// ensurePlan creates one plan and returns its id, or "" on failure.
func (cl *httpClient) ensurePlan(ctx context.Context, rnd *rand.Rand) string {
	body := map[string]any{
		"name":                  fmt.Sprintf("LoadGen Plan %d", rnd.Intn(1_000_000)),
		"monthlyPriceCents":     int64(1000 + rnd.Intn(9000)),
		"includedVoiceMinutes":  100 + rnd.Intn(900),
		"includedDataMB":        1000 + rnd.Intn(9000),
		"includedSMS":           50 + rnd.Intn(450),
		"overageVoiceCents":     int64(5 + rnd.Intn(20)),
		"overageDataCentsPerMB": int64(1 + rnd.Intn(10)),
		"overageSMSCents":       int64(2 + rnd.Intn(8)),
	}
	var out idHolder
	if cl.do(ctx, http.MethodPost, "/v1/plans", body, &out) >= 300 {
		return ""
	}
	return out.ID
}

// createSubscriber creates a subscriber on planID and returns its id, or "".
func (cl *httpClient) createSubscriber(ctx context.Context, rnd *rand.Rand, planID string) string {
	n := rnd.Int63n(1_000_000_000)
	body := map[string]any{
		"msisdn":           fmt.Sprintf("+447%09d", n),
		"imsi":             fmt.Sprintf("234150%09d", n),
		"name":             fmt.Sprintf("Subscriber %d", n),
		"plan_id":          planID,
		"owner_account_id": fmt.Sprintf("acct-%d", rnd.Intn(100)),
	}
	var out idHolder
	if cl.do(ctx, http.MethodPost, "/v1/subscribers", body, &out) >= 300 {
		return ""
	}
	return out.ID
}

// addUsage ingests one random CDR for the subscriber.
func (cl *httpClient) addUsage(ctx context.Context, rnd *rand.Rand, subID string) {
	kinds := []string{"voice", "data", "sms"}
	body := map[string]any{
		"kind":      kinds[rnd.Intn(len(kinds))],
		"quantity":  int64(1 + rnd.Intn(500)),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	cl.do(ctx, http.MethodPost, "/v1/subscribers/"+subID+"/usage", body, nil)
}

// readTraffic issues GETs that touch the read/scenario surface.
func (cl *httpClient) readTraffic(ctx context.Context, rnd *rand.Rand, subID string) {
	cl.do(ctx, http.MethodGet, "/v1/subscribers/"+subID, nil, nil)
	cl.do(ctx, http.MethodGet, "/v1/subscribers/"+subID+"/invoice", nil, nil)
	q := url.Values{}
	q.Set("search", []string{"", "Subscriber", "447"}[rnd.Intn(3)])
	q.Set("limit", strconv.Itoa(10+rnd.Intn(40)))
	q.Set("offset", strconv.Itoa(rnd.Intn(20)))
	cl.do(ctx, http.MethodGet, "/v1/subscribers?"+q.Encode(), nil, nil)
}

// scenarioTraffic occasionally pokes the diagnostics/webhook endpoints so those
// spans/metrics populate. Kept low-frequency to mimic a realistic mix.
func (cl *httpClient) scenarioTraffic(ctx context.Context, rnd *rand.Rand) {
	switch rnd.Intn(2) {
	case 0:
		q := url.Values{}
		q.Set("host", []string{"localhost", "127.0.0.1", "example.com"}[rnd.Intn(3)])
		cl.do(ctx, http.MethodGet, "/v1/diagnostics/ping?"+q.Encode(), nil, nil)
	default:
		body := map[string]any{
			"url": fmt.Sprintf("http://localhost:%d/hook", 9000+rnd.Intn(100)),
		}
		cl.do(ctx, http.MethodPost, "/v1/webhooks", body, nil)
	}
}

// subscriberPool is a concurrency-safe bag of known subscriber ids so workers
// can reuse subscribers for usage/invoice traffic instead of always creating new ones.
type subscriberPool struct {
	mu  sync.Mutex
	ids []string
}

func (p *subscriberPool) add(id string) {
	if id == "" {
		return
	}
	p.mu.Lock()
	p.ids = append(p.ids, id)
	p.mu.Unlock()
}

func (p *subscriberPool) pick(rnd *rand.Rand) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.ids) == 0 {
		return "", false
	}
	return p.ids[rnd.Intn(len(p.ids))], true
}

// runWorker drives one stream of traffic until ctx is cancelled. ticker paces it.
func runWorker(ctx context.Context, cl *httpClient, pool *subscriberPool, planIDs []string, rnd *rand.Rand, tick <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
		}
		// Weighted action mix: mostly usage + reads, some creates, rare scenarios.
		switch n := rnd.Intn(100); {
		case n < 15:
			// create a fresh subscriber on a random existing plan
			plan := planIDs[rnd.Intn(len(planIDs))]
			pool.add(cl.createSubscriber(ctx, rnd, plan))
		case n < 55:
			if id, ok := pool.pick(rnd); ok {
				cl.addUsage(ctx, rnd, id)
			}
		case n < 90:
			if id, ok := pool.pick(rnd); ok {
				cl.readTraffic(ctx, rnd, id)
			}
		default:
			cl.scenarioTraffic(ctx, rnd)
		}
	}
}

func run(cfg config) error {
	st := &stats{}
	cl := newHTTPClient(cfg, st)

	// Honour SIGINT/SIGTERM and the -duration deadline.
	ctx, cancel := context.WithTimeout(context.Background(), cfg.duration)
	defer cancel()
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	seedRnd := rand.New(rand.NewSource(cfg.seed))

	// Seed a handful of plans and an initial subscriber set so workers have
	// something to reference from the first tick.
	var planIDs []string
	for i := 0; i < 3; i++ {
		if id := cl.ensurePlan(ctx, seedRnd); id != "" {
			planIDs = append(planIDs, id)
		}
	}
	if len(planIDs) == 0 {
		return fmt.Errorf("could not create any plan; is the server up at %s and is -key correct? (%s)", cfg.baseURL, st)
	}
	pool := &subscriberPool{}
	for i := 0; i < 5; i++ {
		pool.add(cl.createSubscriber(ctx, seedRnd, planIDs[seedRnd.Intn(len(planIDs))]))
	}

	// One shared ticker fans ticks out to all workers, giving an aggregate rate
	// close to -rps regardless of worker count.
	interval := time.Second / time.Duration(cfg.rps)
	if interval <= 0 {
		interval = time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	workers := cfg.rps
	if workers > 32 {
		workers = 32
	}
	if workers < 1 {
		workers = 1
	}

	log.Printf("loadgen: driving %s at ~%d rps for %s (workers=%d, seed=%d)",
		cfg.baseURL, cfg.rps, cfg.duration, workers, cfg.seed)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		wrnd := rand.New(rand.NewSource(cfg.seed + int64(i) + 1))
		go func() {
			defer wg.Done()
			runWorker(ctx, cl, pool, planIDs, wrnd, ticker.C)
		}()
	}

	// Periodic progress so the operator sees liveness during long runs.
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				log.Printf("loadgen: progress %s", st)
			}
		}
	}()

	<-ctx.Done()
	wg.Wait()
	log.Printf("loadgen: done %s", st)
	return nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("loadgen ")
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		log.Printf("flag error: %v", err)
		os.Exit(2)
	}
	if err := run(cfg); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}
