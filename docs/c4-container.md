# C4 Model - Container Diagram

## Level 2: Container Diagram

```mermaid
C4Container
    title ISO 8583 Payment Simulator - Container Diagram
    
    Person(user, "User", "Web UI or API consumer")
    
    ContainerDb(acquirer_db, "Acquirer PostgreSQL", "PostgreSQL 16", "Stores transaction audit logs")
    ContainerDb(issuer_db, "Issuer PostgreSQL", "PostgreSQL 16", "Stores card data and transaction history")
    
    Container(acquirer, "Acquirer Service", "Go/Gin HTTP API", "HTTP API Gateway - Port 8081")
    Container(issuer, "Issuer Service", "Go/TCP Server", "TCP Backend Service - Port 5001")
    Container(web_ui, "Web UI", "Vanilla JS + Tailwind", "Single-page application")
    
    Container_Ext(grafana, "Grafana Cloud", "Monitoring Platform", "OTLP endpoint")
    
    Rel(user, web_ui, "Uses", "HTTP")
    Rel(user, acquirer, "API calls", "HTTP/JSON")
    
    Rel(web_ui, acquirer, "API calls", "HTTP/JSON")
    
    Rel(acquirer, acquirer_db, "Reads/Writes", "JDBC/PostgreSQL")
    Rel(acquirer, issuer_db, "Reads card data", "JDBC/PostgreSQL")
    Rel(acquirer, issuer, "Forwards transactions", "TCP/ISO8583")
    
    Rel(issuer, issuer_db, "Reads/Writes", "JDBC/PostgreSQL")
    
    Rel(acquirer, grafana, "Sends metrics/traces", "OTLP")
    Rel(issuer, grafana, "Sends metrics/traces", "OTLP")
```

## Container Descriptions

### Acquirer Service
- **Technology**: Go with Gin framework
- **Port**: 8081 (HTTP)
- **Purpose**: HTTP API Gateway that receives transaction requests from clients
- **Responsibilities**:
  - HTTP request handling
  - Transaction validation
  - Backpressure management (rate limiting, queue)
  - Forwarding transactions to Issuer via TCP
  - Storing audit logs in Acquirer PostgreSQL
  - OpenTelemetry metrics and traces

### Issuer Service
- **Technology**: Go with custom TCP server
- **Port**: 5001 (TCP)
- **Purpose**: Backend service that processes transactions and manages card data
- **Responsibilities**:
  - TCP/ISO8583 message processing
  - Card validation and authorization
  - Balance management
  - Transaction processing
  - Storing card data and transactions in Issuer PostgreSQL
  - OpenTelemetry metrics and traces

### Web UI
- **Technology**: Vanilla JavaScript + Tailwind CSS
- **Purpose**: Single-page application for transaction testing
- **Responsibilities**:
  - Transaction form input
  - Card management
  - Transaction history display
  - Real-time message encoding/decoding
  - ISO8583 specification display

### Acquirer PostgreSQL
- **Technology**: PostgreSQL 16
- **Port**: 5433
- **Purpose**: Stores transaction audit logs
- **Data**: Transaction records, timestamps, response codes

### Issuer PostgreSQL
- **Technology**: PostgreSQL 16
- **Port**: 5432
- **Purpose**: Stores card data and transaction history
- **Data**: Card information, balances, transaction records

### Grafana Cloud
- **Technology**: Grafana Cloud
- **Purpose**: Monitoring and observability
- **Data**: Metrics (Prometheus), Traces (Tempo), Logs (Loki)

## Communication Patterns

### HTTP Communication
- **User → Web UI**: HTTP requests for UI assets
- **User → Acquirer**: REST API calls for transactions
- **Web UI → Acquirer**: AJAX calls for transaction processing

### TCP Communication
- **Acquirer → Issuer**: ISO8583 messages over TCP protocol

### Database Communication
- **Acquirer → Acquirer DB**: Read/write audit logs
- **Acquirer → Issuer DB**: Read-only card data access
- **Issuer → Issuer DB**: Read/write card data and transactions

### Observability
- **Acquirer → Grafana**: OTLP metrics and traces
- **Issuer → Grafana**: OTLP metrics and traces
