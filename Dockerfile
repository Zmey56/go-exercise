# Multi-stage build for image size optimization
FROM golang:1.23-alpine AS builder

# Install required packages
RUN apk add --no-cache git ca-certificates tzdata

# Create user for security
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /build

# Copy go.mod for dependency caching
COPY go.mod ./

# Download dependencies (go.sum will be created automatically if needed)
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o server ./cmd/server

# Final image
FROM scratch

# Copy certificates and timezone data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy user
COPY --from=builder /etc/passwd /etc/passwd

# Copy built application
COPY --from=builder /build/server /server

# Switch to non-privileged user
USER appuser

# Expose port
EXPOSE 8080

# Set default environment variables
ENV HTTP_ADDR=:8080
ENV KRAKEN_URL=https://api.kraken.com
ENV CACHE_TTL=60s
ENV HTTP_TIMEOUT=3s

# Run application
ENTRYPOINT ["/server"]
