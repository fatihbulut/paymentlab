## Iterative Tuning with k6

Under heavy load, this service turns overload into deterministic `429`s and timeouts instead of falling over. Your goals when tuning:

- `http_req_failed` should trend down (it's best to report `429`s separately from real failures)
- p95 latency should stay stable
- `iso_parser_in_flight_requests` and `iso_parser_queue_depth` (exposed on `/metrics`) should not grow without bound

### Suggested starting point

1) Start with the defaults:

- `REQUEST_TIMEOUT_MS=1800`
- `INFLIGHT_LIMIT=200`
- `QUEUE_SIZE=500`
- `WORKER_COUNT=100`

2) Run k6 (1000 VUs, 30s). Scrape `/metrics` while the test runs.

### What to change, and when

- **Too many 429s (`queue_full` / `inflight_limit`)**:
  - If there's no downstream bottleneck (there isn't one in this service), raise `WORKER_COUNT` or `INFLIGHT_LIMIT`.
  - If CPU is already at 100%, keep `INFLIGHT_LIMIT` fixed and pull back `WORKER_COUNT` instead — this reduces GC/scheduling pressure.

- **Too many 504s (timeouts)**:
  - First lower `INFLIGHT_LIMIT` / `WORKER_COUNT` (this reduces latency).
  - Only raise `REQUEST_TIMEOUT_MS` as a last resort — otherwise you're just making requests wait longer instead of fixing the root cause.

- **p95 climbing and `queue_depth` rising**:
  - Don't grow `QUEUE_SIZE` — that only lengthens the queue and drives p95 up further.
  - To shed load earlier, lower `INFLIGHT_LIMIT` and/or `QUEUE_SIZE` instead.

### Metrics to watch

- `iso_parser_in_flight_requests`
- `iso_parser_queue_depth`
- `iso_parser_rejected_total{reason=...}`
- `iso_parser_timeouts_total`
- `iso_parser_late_dropped_total`
- `iso_parser_http_request_duration_seconds_bucket{route=...}`
