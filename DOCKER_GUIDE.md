# Docker Deployment Guide

## Overview

This guide explains how to deploy the Bitcoin LTP API with different cache configurations using Docker.

## Quick Start

### 1. Memory Cache Only (Default)
```bash
# Build and run with in-memory cache
docker build -t ltp-api .
docker run -p 8080:8080 ltp-api
```

### 2. Full Stack with Redis (Recommended)
```bash
# Start Redis + API with fallback cache
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f ltp-api
```

### 3. Development Setup
```bash
# Start Redis + API for development
docker-compose -f docker-compose.dev.yml up -d
```

## Cache Configuration Options

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CACHE_TYPE` | `memory` | Cache type: `memory`, `redis`, `fallback` |
| `REDIS_URL` | `redis://localhost:6379` | Redis connection URL |
| `CACHE_DEFAULT_TTL` | `60s` | Default cache TTL |
| `CACHE_FALLBACK_TTL` | `5m` | Fallback cache TTL |
| `CACHE_REFRESH_INTERVAL` | `30s` | Background refresh interval |
| `CACHE_REFRESH_WORKERS` | `3` | Number of refresh workers |
| `CACHE_ENABLE_FALLBACK` | `true` | Enable fallback cache |

### Cache Types

#### 1. Memory Cache
- **Use case**: Development, single instance
- **Pros**: Fast, no external dependencies
- **Cons**: Not shared between instances, lost on restart

```bash
docker run -p 8080:8080 \
  -e CACHE_TYPE=memory \
  ltp-api
```

#### 2. Redis Cache
- **Use case**: Production with Redis infrastructure
- **Pros**: Shared between instances, persistent
- **Cons**: Single point of failure

```bash
docker run -p 8080:8080 \
  -e CACHE_TYPE=redis \
  -e REDIS_URL=redis://your-redis:6379 \
  ltp-api
```

#### 3. Fallback Cache (Recommended)
- **Use case**: Production with high availability
- **Pros**: Best of both worlds, automatic failover
- **Cons**: Slightly more complex

```bash
docker run -p 8080:8080 \
  -e CACHE_TYPE=fallback \
  -e REDIS_URL=redis://your-redis:6379 \
  -e CACHE_ENABLE_FALLBACK=true \
  ltp-api
```

## Docker Compose Files

### `docker-compose.yml` (Production)
- Full stack with Redis
- Fallback cache enabled
- Health checks configured
- Persistent Redis data

### `docker-compose.dev.yml` (Development)
- Same as production but optimized for development
- Faster startup times
- Debug-friendly configuration

### `docker-compose.redis-only.yml` (Testing)
- Redis only for testing cache functionality
- Useful for running cache tests

## Health Checks

### Application Health
```bash
# Check application health
curl http://localhost:8080/health

# Check readiness
curl http://localhost:8080/ready
```

### Redis Health
```bash
# Check Redis directly
docker-compose exec redis redis-cli ping

# Check Redis from application
curl http://localhost:8080/health | jq '.checks[] | select(.name=="cache")'
```

## Monitoring

### Application Metrics
```bash
# Prometheus metrics
curl http://localhost:8080/metrics
```

### Redis Monitoring
```bash
# Redis info
docker-compose exec redis redis-cli info

# Redis memory usage
docker-compose exec redis redis-cli info memory

# Redis keys
docker-compose exec redis redis-cli keys "*"
```

## Troubleshooting

### Common Issues

#### 1. Redis Connection Failed
```bash
# Check Redis status
docker-compose ps redis

# Check Redis logs
docker-compose logs redis

# Test Redis connection
docker-compose exec redis redis-cli ping
```

#### 2. Application Won't Start
```bash
# Check application logs
docker-compose logs ltp-api

# Check health endpoint
curl http://localhost:8080/health
```

#### 3. Cache Not Working
```bash
# Check cache configuration
docker-compose exec ltp-api env | grep CACHE

# Check Redis keys
docker-compose exec redis redis-cli keys "*"

# Test cache endpoint
curl "http://localhost:8080/api/v1/ltp?pair=BTC/USD"
```

### Debug Commands

```bash
# Enter application container
docker-compose exec ltp-api sh

# Enter Redis container
docker-compose exec redis sh

# View all logs
docker-compose logs -f

# Restart services
docker-compose restart

# Clean restart
docker-compose down && docker-compose up -d
```

## Production Deployment

### 1. Environment Configuration
```bash
# Production environment file
cat > .env << EOF
CACHE_TYPE=fallback
REDIS_URL=redis://redis:6379
CACHE_DEFAULT_TTL=60s
CACHE_FALLBACK_TTL=5m
CACHE_REFRESH_INTERVAL=30s
CACHE_REFRESH_WORKERS=5
CACHE_ENABLE_FALLBACK=true
HTTP_ADDR=:8080
KRAKEN_URL=https://api.kraken.com
HTTP_TIMEOUT=3s
EOF
```

### 2. Deploy with Environment File
```bash
docker-compose --env-file .env up -d
```

### 3. Scale Application
```bash
# Scale to 3 instances
docker-compose up -d --scale ltp-api=3
```

### 4. Load Balancer Configuration
```yaml
# nginx.conf example
upstream ltp_api {
    server ltp-api_1:8080;
    server ltp-api_2:8080;
    server ltp-api_3:8080;
}

server {
    listen 80;
    location / {
        proxy_pass http://ltp_api;
    }
}
```

## Security Considerations

### 1. Redis Security
```bash
# Set Redis password
docker run -d --name redis \
  -e REDIS_PASSWORD=your_secure_password \
  redis:7-alpine redis-server --requirepass your_secure_password

# Use password in application
docker run -p 8080:8080 \
  -e REDIS_URL=redis://:your_secure_password@redis:6379 \
  ltp-api
```

### 2. Network Security
```yaml
# docker-compose.yml with networks
version: '3.8'
services:
  redis:
    networks:
      - internal
  ltp-api:
    networks:
      - internal
      - external

networks:
  internal:
    driver: bridge
  external:
    driver: bridge
```

## Performance Tuning

### 1. Redis Configuration
```bash
# Optimize Redis for caching
docker run -d --name redis \
  -e REDIS_MAXMEMORY=256mb \
  -e REDIS_MAXMEMORY_POLICY=allkeys-lru \
  redis:7-alpine
```

### 2. Application Configuration
```bash
# Optimize for high throughput
docker run -p 8080:8080 \
  -e CACHE_REFRESH_WORKERS=10 \
  -e CACHE_REFRESH_INTERVAL=10s \
  ltp-api
```

## Backup and Recovery

### 1. Redis Backup
```bash
# Create backup
docker-compose exec redis redis-cli BGSAVE

# Copy backup
docker cp $(docker-compose ps -q redis):/data/dump.rdb ./backup/
```

### 2. Restore from Backup
```bash
# Stop Redis
docker-compose stop redis

# Copy backup
docker cp ./backup/dump.rdb $(docker-compose ps -q redis):/data/

# Start Redis
docker-compose start redis
```

