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

# Compile binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/worker ./cmd/worker

# Runtime stage: minimal Alpine image
FROM alpine:latest

# Install ca-certificates for HTTPS (SSH targets, Azure Blob Storage uploads)
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Copy binary from builder
COPY --from=builder /app/worker /app/worker

WORKDIR /app

# Set ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# No EXPOSE: the worker does not serve HTTP, it only polls the database.

# Health check: process-based, since there is no HTTP port to probe.
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD pgrep -f /app/worker || exit 1

# Run worker
CMD ["/app/worker"]
