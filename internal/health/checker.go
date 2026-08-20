package health

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/SaltKing0/fujin/internal/store"
)

// Result is a single health-check observation.
type Result struct {
	Endpoint   string
	StatusCode int
	LatencyMs  int64
	Err        error
	CheckedAt  time.Time
}

// Checker periodically pings endpoints and records results.
type Checker struct {
	Endpoints []string
	Interval  time.Duration
	Timeout   time.Duration
	Store     *store.Store
	Client    *http.Client
}

// New creates a default Checker.
func New(endpoints []string, interval time.Duration, store *store.Store) *Checker {
	return &Checker{
		Endpoints: endpoints,
		Interval:  interval,
		Timeout:   10 * time.Second,
		Store:     store,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CheckAll runs one health check against every endpoint, saves results, and
// returns them.
func (c *Checker) CheckAll() []Result {
	results := make([]Result, 0, len(c.Endpoints))
	for _, ep := range c.Endpoints {
		r := c.checkOne(ep)
		if c.Store != nil {
			_ = c.Store.SaveHealthSample(store.HealthSample{
				Endpoint:   ep,
				StatusCode: r.StatusCode,
				LatencyMs:  r.LatencyMs,
				CheckedAt:  r.CheckedAt,
			})
		}
		results = append(results, r)
	}
	return results
}

func (c *Checker) checkOne(endpoint string) Result {
	start := time.Now()
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{Endpoint: endpoint, Err: fmt.Errorf("create request: %w", err), CheckedAt: start}
	}
	req.Header.Set("User-Agent", "kagutsuchi/0.1.0")

	resp, err := c.Client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return Result{Endpoint: endpoint, Err: fmt.Errorf("request failed: %w", err), LatencyMs: latency, CheckedAt: start}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // drain for keep-alive

	return Result{
		Endpoint:   endpoint,
		StatusCode: resp.StatusCode,
		LatencyMs:  latency,
		CheckedAt:  start,
	}
}
