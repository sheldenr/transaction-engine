# ==========================================
# Multi-Stage Dockerfile for Ledger Engine
# Minimal, Secure, Non-Root Alpine Image
# ==========================================

# Stage 1: Build Stage
FROM golang:alpine AS builder

# Install build dependencies and security certificates
RUN apk add --no-cache ca-certificates git tzdata

# Create dedicated non-root user and group
RUN addgroup -S appgroup -g 10001 && \
    adduser -S -u 10001 -G appgroup -h /home/appuser -s /sbin/nologin appuser

WORKDIR /build

# Cache Go modules layer
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source tree
COPY . .

# Build statically linked minimal binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-w -s -extldflags '-static'" \
    -o /bin/ledger-engine ./cmd/server

# Stage 2: Final Minimal Runtime Stage
FROM alpine:3.21

# Upgrade base packages to latest security patch versions
RUN apk --no-cache upgrade

# Import SSL certificates and timezone data from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy compiled binary
COPY --from=builder /bin/ledger-engine /usr/local/bin/ledger-engine

# Use non-root user
USER 10001:10001

# Expose HTTP API port
EXPOSE 8080

# Container Healthcheck
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/ledger-engine", "-healthcheck"] || exit 0

ENTRYPOINT ["/usr/local/bin/ledger-engine"]
