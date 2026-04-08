## K6 ile iteratif tuning

Bu servis aşırı yükte `429` (overload) ve timeout’ları deterministik hale getirir. Hedefiniz:

- `http_req_failed` düşsün (429’ları ayrı raporlamak ideal)
- p95 stabil olsun
- `/metrics` içindeki `iso_parser_in_flight_requests` ve `iso_parser_queue_depth` “sonsuz büyümesin”

### Önerilen başlangıç

1) Varsayılanlarla başlayın:

- `REQUEST_TIMEOUT_MS=1800`
- `INFLIGHT_LIMIT=200`
- `QUEUE_SIZE=500`
- `WORKER_COUNT=100`

2) K6’i çalıştırın (1000 VU, 30s). Test sırasında `/metrics` çekin.

### Ne zaman neyi değiştiriyoruz?

- **Çok fazla 429 (queue_full/inflight_limit)**:
  - Downstream yoksa (bu serviste yok) `WORKER_COUNT` veya `INFLIGHT_LIMIT` artırılabilir.
  - CPU %100’deyse önce `INFLIGHT_LIMIT` sabit tutup `WORKER_COUNT`’u çekin (GC/scheduling baskısını azaltır).

- **Çok fazla 504 (timeout)**:
  - Önce `INFLIGHT_LIMIT` / `WORKER_COUNT` düşürün (latency’yi düşürür).
  - Son çare `REQUEST_TIMEOUT_MS` artırın; aksi halde sadece daha çok bekletmiş olursunuz.

- **p95 artıyor ve queue_depth yükseliyor**:
  - `QUEUE_SIZE` büyütmeyin; bu sadece kuyruğu uzatıp p95’i artırır.
  - Daha erken shed etmek için `INFLIGHT_LIMIT` ve/veya `QUEUE_SIZE` düşürün.

### İzlenecek metrikler

- `iso_parser_in_flight_requests`
- `iso_parser_queue_depth`
- `iso_parser_rejected_total{reason=...}`
- `iso_parser_timeouts_total`
- `iso_parser_late_dropped_total`
- `iso_parser_http_request_duration_seconds_bucket{route=...}`

