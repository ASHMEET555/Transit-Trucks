package app

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
)

var defaultHistogramBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type MetricsCollector struct {
	mu sync.RWMutex

	// Counters: key is "method|route|status"
	httpRequestsTotal map[string]uint64

	// Histograms: key is "method|route"
	latencyCounts  map[string]uint64
	latencySums    map[string]float64
	latencyBuckets map[string][]uint64

	// Events and jobs
	eventsPublished atomic.Uint64
	jobSuccess      atomic.Uint64
	jobFailure      atomic.Uint64

	// In-flight
	inFlight atomic.Int64

	// Domain gauge cache (15 seconds TTL)
	domainMu          sync.Mutex
	domainCachedLines []string
	domainCacheExpiry time.Time
}

func newMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		httpRequestsTotal: make(map[string]uint64),
		latencyCounts:     make(map[string]uint64),
		latencySums:       make(map[string]float64),
		latencyBuckets:    make(map[string][]uint64),
	}
}

func (m *MetricsCollector) IncrementInFlight() {
	m.inFlight.Add(1)
}

func (m *MetricsCollector) DecrementInFlight() {
	m.inFlight.Add(-1)
}

func (m *MetricsCollector) RecordEventPublished() {
	m.eventsPublished.Add(1)
}

func (m *MetricsCollector) RecordJobResult(success bool) {
	if success {
		m.jobSuccess.Add(1)
	} else {
		m.jobFailure.Add(1)
	}
}

// Bounded route mapping: maps arbitrary un-patterned paths to "unmatched"
func boundRoute(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "/" {
		return "root"
	}
	parts := strings.SplitN(pattern, " ", 2)
	if len(parts) == 2 {
		pattern = parts[1]
	}

	// If route starts with /api/v1/ or standard system endpoints, it's bounded
	if strings.HasPrefix(pattern, "/api/v1/") ||
		pattern == "/healthz" || pattern == "/readyz" || pattern == "/metrics" ||
		pattern == "/ws/fleet" {
		return pattern
	}

	return "unmatched"
}

func (m *MetricsCollector) RecordHTTPRequest(method, route string, statusCode int, duration time.Duration) {
	route = boundRoute(route)
	durationSec := duration.Seconds()
	statusStr := strconv.Itoa(statusCode)
	reqKey := fmt.Sprintf("%s|%s|%s", method, route, statusStr)
	histKey := fmt.Sprintf("%s|%s", method, route)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Counter
	m.httpRequestsTotal[reqKey]++

	// 2. Histogram
	m.latencyCounts[histKey]++
	m.latencySums[histKey] += durationSec

	buckets, exists := m.latencyBuckets[histKey]
	if !exists {
		buckets = make([]uint64, len(defaultHistogramBuckets))
		m.latencyBuckets[histKey] = buckets
	}
	for i, b := range defaultHistogramBuckets {
		if durationSec <= b {
			buckets[i]++
		}
	}
}

func (a *App) getCachedDomainGauges(ctx context.Context) []string {
	a.metrics.domainMu.Lock()
	defer a.metrics.domainMu.Unlock()

	if time.Now().Before(a.metrics.domainCacheExpiry) && len(a.metrics.domainCachedLines) > 0 {
		return a.metrics.domainCachedLines
	}

	var lines []string
	if a.db != nil {
		// Vehicles by status
		vRows, err := a.db.Query(ctx, `SELECT status, count(*) FROM vehicles GROUP BY status`)
		if err == nil {
			defer vRows.Close()
			for vRows.Next() {
				var st string
				var cnt int
				if vRows.Scan(&st, &cnt) == nil {
					lines = append(lines, fmt.Sprintf("domain_vehicles_total{status=%q} %d", st, cnt))
				}
			}
		}

		// Trips by status
		tRows, err := a.db.Query(ctx, `SELECT status, count(*) FROM trips GROUP BY status`)
		if err == nil {
			defer tRows.Close()
			for tRows.Next() {
				var st string
				var cnt int
				if tRows.Scan(&st, &cnt) == nil {
					lines = append(lines, fmt.Sprintf("domain_trips_total{status=%q} %d", st, cnt))
				}
			}
		}
	}

	a.metrics.domainCachedLines = lines
	a.metrics.domainCacheExpiry = time.Now().Add(15 * time.Second)
	return lines
}

func (a *App) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// Internal-only: require Bearer token. If missing or invalid, return 404 per spec
	token := ""
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}

	expectedToken := a.cfg.MetricsToken
	if expectedToken == "" && a.cfg.Env != "production" {
		expectedToken = "transitops-metrics-dev"
	}

	if token == "" || expectedToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		w.Header().Set("Content-Type", "application/json")
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Path /metrics not found", nil)
		return
	}

	ctx := r.Context()
	var b strings.Builder

	b.WriteString("# TransitOps Production Prometheus Metrics\n\n")

	// 1. In-flight requests
	b.WriteString("# HELP http_in_flight_requests Current number of HTTP requests being processed.\n")
	b.WriteString("# TYPE http_in_flight_requests gauge\n")
	b.WriteString(fmt.Sprintf("http_in_flight_requests %d\n\n", a.metrics.inFlight.Load()))

	// 2. Events published total
	b.WriteString("# HELP events_published_total Total realtime events emitted via pg_notify.\n")
	b.WriteString("# TYPE events_published_total counter\n")
	b.WriteString(fmt.Sprintf("events_published_total %d\n\n", a.metrics.eventsPublished.Load()))

	// 3. Job executions
	b.WriteString("# HELP job_executions_total Background license check and maintenance job executions.\n")
	b.WriteString("# TYPE job_executions_total counter\n")
	b.WriteString(fmt.Sprintf("job_executions_total{status=\"success\"} %d\n", a.metrics.jobSuccess.Load()))
	b.WriteString(fmt.Sprintf("job_executions_total{status=\"failure\"} %d\n\n", a.metrics.jobFailure.Load()))

	// 4. DB Pool statistics
	if a.db != nil {
		stat := a.db.Stat()
		b.WriteString("# HELP db_pool_total_connections Total connections in pgx pool.\n")
		b.WriteString("# TYPE db_pool_total_connections gauge\n")
		b.WriteString(fmt.Sprintf("db_pool_total_connections %d\n\n", stat.TotalConns()))

		b.WriteString("# HELP db_pool_idle_connections Idle connections in pgx pool.\n")
		b.WriteString("# TYPE db_pool_idle_connections gauge\n")
		b.WriteString(fmt.Sprintf("db_pool_idle_connections %d\n\n", stat.IdleConns()))

		b.WriteString("# HELP db_pool_acquired_connections Currently acquired connections in pgx pool.\n")
		b.WriteString("# TYPE db_pool_acquired_connections gauge\n")
		b.WriteString(fmt.Sprintf("db_pool_acquired_connections %d\n\n", stat.AcquiredConns()))
	}

	// 5. Active WebSocket connections by role
	if a.hub != nil {
		b.WriteString("# HELP ws_connections Number of active WebSocket connections by role.\n")
		b.WriteString("# TYPE ws_connections gauge\n")
		byRole := a.hub.ActiveClientCountByRole()
		if len(byRole) == 0 {
			b.WriteString("ws_connections{role=\"none\"} 0\n")
		} else {
			for role, count := range byRole {
				b.WriteString(fmt.Sprintf("ws_connections{role=%q} %d\n", role, count))
			}
		}
		b.WriteString("\n")
	}

	// 6. Domain gauges (cached 15s)
	domainLines := a.getCachedDomainGauges(ctx)
	if len(domainLines) > 0 {
		b.WriteString("# HELP domain_vehicles_total Total vehicles in fleet by status (cached 15s).\n")
		b.WriteString("# TYPE domain_vehicles_total gauge\n")
		b.WriteString("# HELP domain_trips_total Total trips in fleet by status (cached 15s).\n")
		b.WriteString("# TYPE domain_trips_total gauge\n")
		for _, line := range domainLines {
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	// 7. HTTP requests total & latency histogram
	a.metrics.mu.RLock()
	b.WriteString("# HELP http_requests_total Total number of HTTP requests partitioned by method, route, and status.\n")
	b.WriteString("# TYPE http_requests_total counter\n")
	for key, count := range a.metrics.httpRequestsTotal {
		parts := strings.Split(key, "|")
		if len(parts) == 3 {
			b.WriteString(fmt.Sprintf("http_requests_total{method=%q,route=%q,status=%q} %d\n", parts[0], parts[1], parts[2], count))
		}
	}
	b.WriteString("\n")

	b.WriteString("# HELP http_request_duration_seconds HTTP request latency histogram partitioned by method and route.\n")
	b.WriteString("# TYPE http_request_duration_seconds histogram\n")
	for key, count := range a.metrics.latencyCounts {
		parts := strings.Split(key, "|")
		if len(parts) != 2 {
			continue
		}
		method, route := parts[0], parts[1]
		sum := a.metrics.latencySums[key]
		buckets := a.metrics.latencyBuckets[key]

		for i, bound := range defaultHistogramBuckets {
			b.WriteString(fmt.Sprintf("http_request_duration_seconds_bucket{method=%q,route=%q,le=\"%g\"} %d\n", method, route, bound, buckets[i]))
		}
		b.WriteString(fmt.Sprintf("http_request_duration_seconds_bucket{method=%q,route=%q,le=\"+Inf\"} %d\n", method, route, count))
		b.WriteString(fmt.Sprintf("http_request_duration_seconds_sum{method=%q,route=%q} %g\n", method, route, sum))
		b.WriteString(fmt.Sprintf("http_request_duration_seconds_count{method=%q,route=%q} %d\n", method, route, count))
	}
	a.metrics.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b.String()))
}
