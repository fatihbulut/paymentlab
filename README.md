# iso-parser-service

Gin tabanlı ISO8583 `pack` / `parse` servisi.

## Endpoints

- `POST /v1/parse` `{ "raw_hex": "..." }` → ISO8583 JSON
- `POST /v1/pack` `ISOMessage` JSON → `{ hex, length, details }`
- `GET /healthz`
- `GET /metrics` (Prometheus text format)

## Load altında stabilite (backpressure + timeout)

Bu servis, aşırı yükte sınırsız beklemek yerine kontrollü şekilde reddeder:

- **In-flight limit**: `INFLIGHT_LIMIT` (default `200`) aşıldığında `429` + `Retry-After: 1`
- **Queue limit**: `QUEUE_SIZE` (default `500`) dolduğunda `429` + `Retry-After: 1`
- **Request timeout**: `REQUEST_TIMEOUT_MS` (default `1800`) dolduğunda `504`

### Config (env)

- `PORT` (default `8080`)
- `REQUEST_TIMEOUT_MS` (default `1800`)
- `INFLIGHT_LIMIT` (default `200`)
- `QUEUE_SIZE` (default `500`)
- `WORKER_COUNT` (default `100`)
- `MAX_BODY_BYTES` (default `1048576`)
- `LOG_REQUESTS` (`true/false`, default `false`)
- `READ_HEADER_TIMEOUT_MS` (default `5000`)
- `READ_TIMEOUT_MS` (default `10000`)
- `WRITE_TIMEOUT_MS` (default `2500`)
- `IDLE_TIMEOUT_MS` (default `60000`)
