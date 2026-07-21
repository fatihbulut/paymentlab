# ISO 8583 Payment Simulator

## Quick Start

```bash
# Clone the repository
git clone https://github.com/fatihbulut/paymentlab.git
cd paymentlab

# Start all services
docker compose up -d

# Access the web UI
http://localhost:8081
```

## Services

- **Acquirer**: HTTP API (port 8081)
- **Issuer**: TCP Service (port 5001)
- **PostgreSQL**: Database (ports 5432, 5433)

## Configuration

Copy example environment files and configure:

```bash
cp .env.example .env
# Edit .env with your settings
```

## Production Deployment

For production deployment on separate servers:

```bash
# On issuer server
cp .env.issuer.example ~/.env
docker compose -f docker-compose.issuer.yml up -d

# On acquirer server
cp .env.acquirer.example ~/.env
docker compose -f docker-compose.acquirer.yml up -d
```

## Documentation

- [Processing Codes](docs/PROCESSING_CODES.md)
- [Transaction Scenarios](docs/TRANSACTION_SCENARIOS.md)
