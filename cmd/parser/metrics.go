package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	inFlight   atomic.Int64
	queueDepth atomic.Int64

	rejectedInFlight atomic.Uint64
	rejectedQueue    atomic.Uint64
	timeoutsTotal    atomic.Uint64
	lateDroppedTotal atomic.Uint64

	// keyed by route template (e.g. /v1/parse)
	perRoute sync.Map // map[string]*RouteMetrics
}

type RouteMetrics struct {
	requestsTotal atomic.Uint64
	errorsTotal   atomic.Uint64

	status2xx atomic.Uint64
	status4xx atomic.Uint64
	status5xx atomic.Uint64

	latencyCount atomic.Uint64
	latencySumNS atomic.Uint64
	latencyBuck  []atomic.Uint64
}

var latencyBuckets = []time.Duration{
	50 * time.Millisecond,
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) IncInFlight() { m.inFlight.Add(1) }
func (m *Metrics) DecInFlight() { m.inFlight.Add(-1) }
func (m *Metrics) InFlight() int64 {
	return m.inFlight.Load()
}

func (m *Metrics) IncQueueDepth() { m.queueDepth.Add(1) }
func (m *Metrics) DecQueueDepth() { m.queueDepth.Add(-1) }
func (m *Metrics) QueueDepth() int64 {
	return m.queueDepth.Load()
}

func (m *Metrics) IncRejectedInFlight() { m.rejectedInFlight.Add(1) }
func (m *Metrics) IncRejectedQueue()    { m.rejectedQueue.Add(1) }
func (m *Metrics) IncTimeout()          { m.timeoutsTotal.Add(1) }
func (m *Metrics) IncLateDropped()      { m.lateDroppedTotal.Add(1) }

func (m *Metrics) Observe(route string, status int, dur time.Duration) {
	rm := m.getRoute(route)

	rm.requestsTotal.Add(1)

	switch {
	case status >= 200 && status < 300:
		rm.status2xx.Add(1)
	case status >= 400 && status < 500:
		rm.status4xx.Add(1)
		rm.errorsTotal.Add(1)
	case status >= 500:
		rm.status5xx.Add(1)
		rm.errorsTotal.Add(1)
	}

	rm.latencyCount.Add(1)
	rm.latencySumNS.Add(uint64(dur.Nanoseconds()))
	for i, b := range latencyBuckets {
		if dur <= b {
			rm.latencyBuck[i].Add(1)
			return
		}
	}
	// +Inf bucket is implicit: count - sum(buckets)
}

func (m *Metrics) getRoute(route string) *RouteMetrics {
	if route == "" {
		route = "unknown"
	}
	if v, ok := m.perRoute.Load(route); ok {
		return v.(*RouteMetrics)
	}

	rm := &RouteMetrics{
		latencyBuck: make([]atomic.Uint64, len(latencyBuckets)),
	}
	actual, _ := m.perRoute.LoadOrStore(route, rm)
	return actual.(*RouteMetrics)
}

// ServePrometheus writes a minimal Prometheus text exposition.
func (m *Metrics) ServePrometheus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	var routes []string
	m.perRoute.Range(func(k, _ any) bool {
		routes = append(routes, k.(string))
		return true
	})
	sort.Strings(routes)

	sb := &strings.Builder{}
	writeHelpType(sb, "iso_parser_in_flight_requests", "Current in-flight HTTP requests.", "gauge")
	fmt.Fprintf(sb, "iso_parser_in_flight_requests %d\n", m.InFlight())

	writeHelpType(sb, "iso_parser_queue_depth", "Current internal work queue depth.", "gauge")
	fmt.Fprintf(sb, "iso_parser_queue_depth %d\n", m.QueueDepth())

	writeHelpType(sb, "iso_parser_rejected_total", "Total rejected requests due to overload.", "counter")
	fmt.Fprintf(sb, "iso_parser_rejected_total{reason=\"inflight_limit\"} %d\n", m.rejectedInFlight.Load())
	fmt.Fprintf(sb, "iso_parser_rejected_total{reason=\"queue_full\"} %d\n", m.rejectedQueue.Load())

	writeHelpType(sb, "iso_parser_timeouts_total", "Total request timeouts (server-side).", "counter")
	fmt.Fprintf(sb, "iso_parser_timeouts_total %d\n", m.timeoutsTotal.Load())

	writeHelpType(sb, "iso_parser_late_dropped_total", "Total jobs dropped because request was already cancelled.", "counter")
	fmt.Fprintf(sb, "iso_parser_late_dropped_total %d\n", m.lateDroppedTotal.Load())

	writeHelpType(sb, "iso_parser_http_requests_total", "Total HTTP requests by route.", "counter")
	writeHelpType(sb, "iso_parser_http_errors_total", "Total HTTP error responses (4xx+5xx) by route.", "counter")
	writeHelpType(sb, "iso_parser_http_responses_total", "Total HTTP responses by class (2xx/4xx/5xx) and route.", "counter")
	writeHelpType(sb, "iso_parser_http_request_duration_seconds", "HTTP request duration seconds by route.", "histogram")

	for _, route := range routes {
		rm, _ := m.perRoute.Load(route)
		mr := rm.(*RouteMetrics)

		labels := fmt.Sprintf(`route=%s`, strconv.Quote(route))
		fmt.Fprintf(sb, "iso_parser_http_requests_total{%s} %d\n", labels, mr.requestsTotal.Load())
		fmt.Fprintf(sb, "iso_parser_http_errors_total{%s} %d\n", labels, mr.errorsTotal.Load())
		fmt.Fprintf(sb, "iso_parser_http_responses_total{%s,class=\"2xx\"} %d\n", labels, mr.status2xx.Load())
		fmt.Fprintf(sb, "iso_parser_http_responses_total{%s,class=\"4xx\"} %d\n", labels, mr.status4xx.Load())
		fmt.Fprintf(sb, "iso_parser_http_responses_total{%s,class=\"5xx\"} %d\n", labels, mr.status5xx.Load())

		// histogram
		var cumulative uint64
		for i, b := range latencyBuckets {
			cumulative += mr.latencyBuck[i].Load()
			fmt.Fprintf(sb, "iso_parser_http_request_duration_seconds_bucket{%s,le=%q} %d\n", labels, fmt.Sprintf("%.3f", b.Seconds()), cumulative)
		}
		// +Inf bucket
		fmt.Fprintf(sb, "iso_parser_http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", labels, mr.latencyCount.Load())
		fmt.Fprintf(sb, "iso_parser_http_request_duration_seconds_sum{%s} %.9f\n", labels, float64(mr.latencySumNS.Load())/1e9)
		fmt.Fprintf(sb, "iso_parser_http_request_duration_seconds_count{%s} %d\n", labels, mr.latencyCount.Load())
	}

	_, _ = w.Write([]byte(sb.String()))
}

func writeHelpType(sb *strings.Builder, name, help, typ string) {
	fmt.Fprintf(sb, "# HELP %s %s\n", name, help)
	fmt.Fprintf(sb, "# TYPE %s %s\n", name, typ)
}
