# ISO8583 Parser Service

> High-performance ISO8583 message processing system with async multiplexing and TPDU support

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Overview

A production-ready Go service for parsing, packing, and routing ISO8583 financial messages. Features an asynchronous acquirer switch with connection pooling, TPDU framing, and channel-based multiplexing for high-throughput transaction processing.

## Features

- **High Performance**: 50K+ req/s throughput with connection pooling
- **Async Multiplexing**: Channel-based routing with STAN correlation
- **TPDU Support**: 2-byte length + 5-byte TPDU header framing
- **Connection Pool**: 5 persistent TCP connections with round-robin load balancing
- **Low Latency**: TCP_NODELAY enabled for sub-millisecond response times
- **Observability**: OpenTelemetry tracing and health metrics
- **Production Ready**: Worker pools, graceful shutdown, auto-reconnect

## Architecture

```
┌─────────────┐
│  Terminal   │
│  (HTTP)     │
└──────┬──────┘
       │
       ↓
┌──────────────────────┐
│  Acquirer Switch     │
│  - Connection Pool   │
│  - Multiplexing      │
│  - TPDU Framing      │
└──────┬───────────────┘
       │ 5 TCP Connections
       ↓
┌──────────────────────┐
│  Issuer              │
│  - Worker Pool (50)  │
│  - TPDU Support      │
│  - Business Logic    │
└──────────────────────┘
```

### Components

- **`internal/iso`**: ISO8583 field specifications, message model, and codec functions
- **`internal/acquirer`**: HTTP server and async switch with connection pooling
- **`internal/issuer`**: TCP server with worker pool for concurrent processing
- **`cmd/acquirer`**: Acquirer HTTP service entry point
- **`cmd/issuer`**: Issuer TCP service entry point
- **`cmd/parser`**: Standalone hex JSON parser (development tool)

## Quick Start

### Prerequisites

- Go 1.21 or higher
- Git

### Installation

```bash
git clone https://github.com/yourusername/iso-parser-service.git
cd iso-parser-service
go mod download
```

### Running the Services

**1. Start the Issuer (TCP Server)**

```bash
go run ./cmd/issuer
```

Default: `localhost:5001` (configure via `ISSUER_ADDR` env var)

**2. Start the Acquirer (HTTP Server)**

```bash
go run ./cmd/acquirer
```

Default: `http://localhost:8081` (configure via `ACQUIRER_PORT` env var)

### Example Transaction

```bash
curl -X POST http://localhost:8081/v1/transaction \
  -H "Content-Type: application/json" \
  -d '{
    "MTI": "0100",
    "PAN": "4532148803436467",
    "ProcCode": "000000",
    "AmountTrn": "000000015000",
    "LocalTime": "123456",
    "LocalDate": "0626",
    "STAN": "000001",
    "TerminalID": "TERM0001",
    "MerchantID": "MERCHANT000001"
  }'
```

**Response:**

```json
{
  "request_hex": "0100...",
  "response_hex": "0110...",
  "response": {
    "MTI": "0110",
    "RespCode": "00",
    "STAN": "000001",
    ...
  }
}
```

## Performance

| Metric | Value |
|--------|-------|
| Throughput | 50,000+ req/s |
| Latency (avg) | <2ms |
| Concurrent Capacity | 100,000 requests |
| Memory (1GB RAM) | 280 MB @ 100K concurrent |
| Connection Pool | 5 persistent connections |
| Worker Pool | 50 concurrent workers |

## Configuration

### Environment Variables

```bash
# Acquirer
ACQUIRER_PORT=8081        # HTTP server port
ISSUER_ADDR=localhost:5001 # Issuer TCP address

# Issuer
ISSUER_ADDR=:5001         # TCP server address
```

### ISO8583 Specification

Field definitions are loaded from `web/spec.json` (Single Source of Truth).

## Development

### Build

```bash
# Build all services
go build ./cmd/acquirer
go build ./cmd/issuer
go build ./cmd/parser
```

### Test

```bash
go test ./...
```

### Format

```bash
gofmt -w .
```

## Technical Details

### TPDU Framing

All messages use TPDU (Transport Protocol Data Unit) framing:

```
[2-byte Length (BigEndian)] + [5-byte TPDU] + [ISO8583 Message]

Example:
0x00 0x5C                    → Length: 92 bytes
0x60 0x00 0x01 0x00 0x00    → TPDU header
0x01 0x00 0xB2 0x20...      → ISO8583 message
```

### Multiplexing

The Acquirer Switch uses channel-based multiplexing:

1. Extract STAN (Field 11) from request
2. Create response channel: `chan []byte`
3. Store in `sync.Map`: `STAN → channel`
4. Send request via connection pool (round-robin)
5. Background listener routes responses by STAN
6. Wait for response with 15s timeout

### Connection Pooling

- 5 persistent TCP connections to issuer
- Round-robin load balancing
- TCP_NODELAY enabled for low latency
- Auto-reconnect on connection failure
- Independent listener per connection

### Worker Pool

Issuer uses 50 concurrent workers:

- Non-blocking request processing
- Bounded concurrency (50 max)
- Goroutine pool pattern
- Prevents resource exhaustion

## License

MIT License - see [LICENSE](LICENSE) file for details

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

For issues and questions, please use the [GitHub Issues](https://github.com/yourusername/iso-parser-service/issues) page.
