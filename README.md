# PaymentLab

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker)
![ISO8583](https://img.shields.io/badge/ISO-8583-orange)
![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-enabled-7A3E9D?logo=opentelemetry)

**PaymentLab** is a microservice-based ISO 8583 payment simulator that models a realistic acquirer/issuer payment flow — built for testing, learning, and prototyping against ISO 8583 financial messaging without touching a real card network.

## Overview

The project simulates the two sides of a card transaction:

- **Acquirer** — an HTTP API gateway (and web UI) that receives transaction requests, applies backpressure/rate limiting, and forwards them to the issuer over TCP using ISO 8583-encoded messages.
- **Issuer** — a TCP backend that decodes ISO 8583 messages, authorizes transactions against card/balance data, and returns a response.

Both services are independent Go binaries, each with their own PostgreSQL database, and can be run together locally with a single `docker compose up`, or deployed as two separate services (e.g. on different hosts) using the split compose files.

## Screenshots

### Transaction Simulator

<img width="2860" height="1512" alt="image" src="https://github.com/user-attachments/assets/36b72c31-5733-464a-b627-22da9a24ef75" />

*Build ISO 8583 requests, inspect parsed fields, and trace issuer responses.*

### Webhook Integration

<img width="2838" height="1506" alt="image" src="https://github.com/user-attachments/assets/1868fa5d-a0d8-4d52-b19f-8b70a2d7e809" />

*Receive signed webhook events with automatic retries and delivery history.*

## Architecture

```
                    ┌─────────────┐
   User / Web UI ──▶│  Acquirer   │  HTTP API · Port 8081
                    │  (Go/Gin)   │
                    └──────┬──────┘
                           │ ISO 8583 over TCP
                           ▼
                    ┌─────────────┐
                    │   Issuer    │  TCP Server · Port 5001
                    │    (Go)     │
                    └──────┬──────┘
                           │
              ┌────────────┴────────────┐
              ▼                         ▼
     Acquirer PostgreSQL         Issuer PostgreSQL
     (audit logs, port 5433)     (cards, balances, port 5432)
```

Full C4 model diagrams (context / container / component level) are in [`docs/`](docs/):

- [System Context](docs/c4-context.md)
- [Container Diagram](docs/c4-container.md)
- [Component Diagram](docs/c4-component.md)

## Features

- Interactive web interface for building and inspecting ISO 8583 messages
- ISO 8583 message encoding/decoding driven by a declarative spec ([`web/spec.json`](web/spec.json)) — no hardcoded field layouts
- Acquirer ↔ Issuer flow over raw TCP, mirroring how real payment switches communicate
- Card management API (create, list, update, delete, top-up)
- Transaction processing with configurable backpressure (in-flight limits, queueing, timeouts) so overload degrades predictably (`429`/`504`) instead of falling over
- Full transaction and card audit trail in PostgreSQL, with separate acquirer/issuer databases
- Interactive web UI for building, sending, and tracing ISO 8583 messages in real time
- OpenTelemetry metrics and tracing, OTLP-exportable to any compatible backend (optional — see [`docs/OBSERVABILITY.md`](docs/OBSERVABILITY.md) for a Grafana Cloud walkthrough)
- Built-in `/health`, `/healthz`, and `/metrics` endpoints

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) (quick start / recommended)
- [Go 1.25+](https://go.dev/dl/) (only needed for local, non-Docker development)
- [PostgreSQL 16](https://www.postgresql.org/) (only needed for local, non-Docker development — Docker Compose provisions this for you)

## Quick Start

```bash
git clone https://github.com/fatihbulut/paymentlab.git
cd paymentlab

cp .env.example .env

docker compose up -d --build
```

Then open the web UI at **http://localhost:8081**.

| Service | Address |
|---|---|
| Acquirer HTTP API / Web UI | http://localhost:8081 |
| Issuer TCP service | localhost:5001 |
| Acquirer PostgreSQL | localhost:5433 |
| Issuer PostgreSQL | localhost:5432 |

To stop everything:

```bash
docker compose down          # keep data
docker compose down -v       # also wipe database volumes
```

## Docker Compose Usage

This repo ships three compose files:

- **`docker-compose.yml`** — the all-in-one local development stack: builds both services from source and runs them alongside their own Postgres instances. This is what `docker compose up` uses by default and is the right choice for local development and evaluation.
- **`docker-compose.acquirer.yml`** / **`docker-compose.issuer.yml`** — an example of running acquirer and issuer as independent deployments (e.g. on two separate hosts), pulling prebuilt images instead of building from source. See the comments at the top of each file for usage, and copy the matching `.env.acquirer.example` / `.env.issuer.example` before use.

## Local Development

Running the services directly with Go is faster for iterating on code than rebuilding containers each time.

1. Start just the databases:

   ```bash
   docker compose up -d issuer_postgres acquirer_postgres
   ```

2. Set the environment variables from `.env.example` (source them into your shell, or configure your editor's run/debug configuration).

3. Run each service in its own terminal:

   ```bash
   go run ./cmd/issuer
   go run ./cmd/acquirer
   ```

Database schema is managed via plain SQL migrations in [`migrations/`](migrations/) (see `acquirer/` and `issuer/` subfolders); they're applied automatically on service startup.

## Testing

Run the Go unit tests:

```bash
go test ./...
```

For a load test against a running acquirer instance (uses [Vegeta](https://github.com/tsenart/vegeta)):

```bash
docker compose up -d --build
./test/load-test.sh
```

See [`docs/loadtest.md`](docs/loadtest.md) for guidance on tuning backpressure settings (`INFLIGHT_LIMIT`, `QUEUE_SIZE`, timeouts) under load.

## Project Structure

```
cmd/
  acquirer/          # Acquirer service entrypoint
  issuer/             # Issuer service entrypoint
internal/
  acquirer/           # HTTP handlers, middleware, TCP client to issuer
  auth/                # Authorization rules and response codes
  card/                # Card domain model and service
  config/              # Environment-based configuration
  iso/                 # ISO 8583 codec + spec loader
  issuer/              # TCP server and transaction processing
  otel/                # OpenTelemetry setup and custom metrics
  proccode/            # ISO 8583 processing code (field 3) parsing
  scheme/              # BIN ranges, EMV tags, scheme response codes
  store/               # Storage interfaces + PostgreSQL implementation
  util/                # Shared helpers
migrations/            # SQL schema migrations (shared, acquirer, issuer)
web/                   # Static web UI + ISO 8583 spec (spec.json)
docs/                  # Architecture diagrams and domain documentation
test/                  # Load test script and sample request payload
```

## Documentation

- [Processing Codes (Field 3) Guide](docs/PROCESSING_CODES.md)
- [Transaction Scenarios Guide](docs/TRANSACTION_SCENARIOS.md)
- [Load Testing & Tuning](docs/loadtest.md)
- [Observability / OTLP Setup](docs/OBSERVABILITY.md)
- [C4 Model Diagrams](docs/c4-context.md) ([container](docs/c4-container.md), [component](docs/c4-component.md))

## Contributing

Contributions are welcome!

1. Fork the repo and create a feature branch.
2. Make your changes, keeping them focused and consistent with the existing code style.
3. Add or update tests where relevant (`go test ./...` should pass).
4. Run `go vet ./...` and make sure the CI workflow would pass.
5. Open a pull request describing what changed and why.

Please don't commit secrets, credentials, or environment files (`.env`) — only the `.env*.example` templates belong in the repo.

## Roadmap

_TBD — contributions and suggestions welcome. Open an issue to propose a direction._

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
