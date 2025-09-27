# Golang Developer Assigment

Develop in Go language a service that will provide an API for retrieval of the Last Traded Price of Bitcoin for the following currency pairs:

1. BTC/USD
2. BTC/CHF
3. BTC/EUR


The request path is:
/api/v1/ltp

The response shall constitute JSON of the following structure:
```json
{
  "ltp": [
    {
      "pair": "BTC/CHF",
      "amount": 49000.12
    },
    {
      "pair": "BTC/EUR",
      "amount": 50000.12
    },
    {
      "pair": "BTC/USD",
      "amount": 52000.12
    }
  ]
}

```

# Requirements:

1. The incoming request can done for as for a single pair as well for a list of them
2. You shall provide time accuracy of the data up to the last minute.
3. Code shall be hosted in a remote public repository
4. readme.md includes clear steps to build and run the app
5. Integration tests
6. Dockerized application

# Additional Requirements:

1. Add /health (liveness) and /ready (readiness that checks cache warmness and Kraken reachability). Expose /metrics with Prometheus counters/histograms for upstream calls and cache hits/misses.
2. Implement token-bucket rate limiting (per-IP and global) + exponential backoff with jitter for 429/5xx from Kraken; export limiter metrics and add tests that simulate throttling.
3. Provide a Redis cache adapter (config-switchable) and an in-memory fallback; show TTL expiry, eviction, and background refresh tests.
4. Publish go test -cover ./... results and add explicit failure-path tests (network timeout, bad payloads, partial pair failures).
5. Ship minimal OpenAPI (Swagger) for /api/v1/ltp with schemas for success and partial-success.

# Build and Run Instructions

## Prerequisites
- Go 1.23 or higher
- Docker (optional)

## Local Build and Run

### 1. Clone the repository
```bash
git clone <repository-url>
cd go-exercise
```

### 2. Download dependencies
```bash
go mod download
```

### 3. Build the application
```bash
go build -o server ./cmd/server
```

### 4. Run the server
```bash
./server
```

The server will be available at `http://localhost:8080`

## Environment Variables

You can customize the application behavior using these environment variables:

- `HTTP_ADDR` - Server listening address (default: `:8080`)
- `KRAKEN_URL` - Kraken API base URL (default: `https://api.kraken.com`)
- `CACHE_TTL` - Cache time-to-live (default: `60s`)
- `HTTP_TIMEOUT` - HTTP request timeout (default: `3s`)

Example:
```bash
HTTP_ADDR=:9000 CACHE_TTL=30s ./server
```

## Docker Deployment

### Build Docker image
```bash
docker build -t ltp-api .
```

### Run container (memory cache only)
```bash
docker run -p 8080:8080 ltp-api
```

### Using docker-compose (with Redis)
```bash
# Full stack with Redis cache
docker-compose up -d

# Development with Redis
docker-compose -f docker-compose.dev.yml up -d

# Redis only (for testing)
docker-compose -f docker-compose.redis-only.yml up -d
```

### Cache Configuration Options

The application supports multiple cache configurations:

#### Memory Cache (Default)
```bash
docker run -p 8080:8080 \
  -e CACHE_TYPE=memory \
  ltp-api
```

#### Redis Cache
```bash
docker run -p 8080:8080 \
  -e CACHE_TYPE=redis \
  -e REDIS_URL=redis://localhost:6379 \
  ltp-api
```

#### Fallback Cache (Recommended for Production)
```bash
docker run -p 8080:8080 \
  -e CACHE_TYPE=fallback \
  -e REDIS_URL=redis://localhost:6379 \
  -e CACHE_ENABLE_FALLBACK=true \
  ltp-api
```

## Testing

### Run all tests
```bash
go test ./...
```

### Run tests with coverage
```bash
go test -cover ./...
```

### Run tests with verbose output
```bash
go test -v ./...
```

## API Usage

### Get all supported pairs
```bash
curl http://localhost:8080/api/v1/ltp
```

### Get single pair
```bash
curl "http://localhost:8080/api/v1/ltp?pair=BTC/USD"
```

### Get multiple pairs
```bash
curl "http://localhost:8080/api/v1/ltp?pairs=BTC/USD,BTC/EUR,BTC/CHF"
```

### Expected Response Format
```json
{
  "ltp": [
    {
      "pair": "BTC/CHF",
      "amount": 49000.12
    },
    {
      "pair": "BTC/EUR",
      "amount": 50000.12
    },
    {
      "pair": "BTC/USD",
      "amount": 52000.12
    }
  ]
}
```

For a machine-readable contract, see the minimal OpenAPI description at `docs/openapi.yaml`.

## Architecture

The project follows clean architecture principles with separation of concerns:

- `cmd/server` - Application entry point
- `internal/httpserver` - HTTP server and handlers
- `internal/ltp` - Business logic for LTP service
- `internal/kraken` - Kraken API client
- `internal/cache` - Data caching layer
- `internal/pairs` - Currency pair utilities
- `internal/resp` - HTTP response utilities

## Features

- Real-time Bitcoin price data from Kraken API
- 1-minute data caching for optimal performance
- Support for single and multiple currency pair requests
- CORS support for web applications
- Graceful shutdown on SIGINT/SIGTERM
- Comprehensive error handling with appropriate HTTP status codes
- Input validation and sanitization
- Request logging
- Panic recovery middleware

# API Documentation
The public Kraken API is used to retrieve the LTP information
[Kraken API Documentation](https://docs.kraken.com/rest/#tag/Spot-Market-Data/operation/getTickerInformation)
(The last traded price value is called "last trade closed")
