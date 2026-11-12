# Multi-stage build: builder + runtime
# Builder stage: compile Go binary
FROM golang:1.27-alpine AS builder

WORKDIR /build

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy entire source
COPY . .

# Compile binary (VERSION is accepted for consistency with API build, but not used)
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/worker ./cmd/worker

# Runtime stage: minimal Alpine image
FROM alpine:latest

# ca-certificates for HTTPS. docker-cli talks to the host daemon when a
# server uses connectionMode=local. su-exec drops root after the entrypoint
# joins appuser to the socket's group.
RUN apk --no-cache add ca-certificates docker-cli su-exec

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Copy binary from builder
COPY --from=builder /app/worker /app/worker
COPY docker-entrypoint.sh /docker-entrypoint.sh

WORKDIR /app

# Set ownership. The entrypoint starts as root and execs the binary as appuser.
RUN chown -R appuser:appuser /app && chmod 755 /docker-entrypoint.sh

ENTRYPOINT ["/docker-entrypoint.sh"]

# Set timezone to America/Sao_Paulo for consistent cron scheduling.
# The Go binary embeds tzdata via `import _ "time/tzdata"`, so this does not
# require tzdata package to be installed in the image.
ENV TZ=America/Sao_Paulo

# No EXPOSE: the worker does not serve HTTP, it only polls the database.

# Health check: process-based, since there is no HTTP port to probe.
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD pgrep -f /app/worker || exit 1

# Run worker
CMD ["/app/worker"]
