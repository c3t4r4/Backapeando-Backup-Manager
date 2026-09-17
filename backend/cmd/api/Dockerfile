# Multi-stage build: builder + runtime
# Builder stage: compile Go binary
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy entire source
COPY . .

# Compile binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/api ./cmd/api

# Runtime stage: minimal Alpine image
FROM alpine:latest

# Install ca-certificates for HTTPS (if needed)
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Copy binary from builder
COPY --from=builder /app/api /app/api

WORKDIR /app

# Set ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Set timezone to America/Sao_Paulo for consistent cron scheduling.
# The Go binary embeds tzdata via `import _ "time/tzdata"`, so this does not
# require tzdata package to be installed in the image.
ENV TZ=America/Sao_Paulo

# Expose port
EXPOSE 8081

# Health check: every 30 seconds, timeout 5s, 3 retries
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -q -O- http://localhost:8081/api/health || exit 1

# Run API server
CMD ["/app/api"]
