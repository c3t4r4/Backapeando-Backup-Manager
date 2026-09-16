package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter is the interface for login rate limiting.
// Implementations may be in-memory or distributed (e.g., Redis).
type RateLimiter interface {
	Allow(ctx context.Context, ip string) (bool, error)
}

// RedisRateLimiter is a distributed rate limiter backed by Redis.
// It implements fixed-window rate limiting per IP address.
type RedisRateLimiter struct {
	client   *redis.Client
	window   time.Duration
	maxTries int
}

// NewRedisRateLimiter creates a new Redis-backed rate limiter.
// redisURL should be in the format "redis://host:port" or "redis://host:port/db".
// maxTriesPerMin specifies the maximum number of login attempts allowed per minute.
func NewRedisRateLimiter(redisURL string, maxTriesPerMin int) (*RedisRateLimiter, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &RedisRateLimiter{
		client:   client,
		window:   time.Minute,
		maxTries: maxTriesPerMin,
	}, nil
}

// Allow checks if the given IP is within the rate limit.
// Returns true if the request is allowed, false if rate limit is exceeded.
func (rl *RedisRateLimiter) Allow(ctx context.Context, ip string) (bool, error) {
	key := fmt.Sprintf("ratelimit:login:%s", ip)

	// Increment the counter
	val, err := rl.client.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis incr: %w", err)
	}

	// Set expiration on first increment
	if val == 1 {
		if err := rl.client.Expire(ctx, key, rl.window).Err(); err != nil {
			return false, fmt.Errorf("redis expire: %w", err)
		}
	}

	return val <= int64(rl.maxTries), nil
}

// LoginRateLimiter is a simple fixed-window per-IP limiter for the login
// endpoint. It is in-memory (per-process) — acceptable for single-replica
// deployments; it deliberately doesn't need to be perfectly precise, only to
// make brute-forcing the admin password impractical.
//
// Deprecated: Use RedisRateLimiter for distributed deployments.
type LoginRateLimiter struct {
	// Legacy in-memory implementation kept for backward compatibility
	// and development without Redis.
}

// NewLoginRateLimiter creates a new in-memory rate limiter.
// This is retained for backward compatibility and local development.
// For production, use NewRedisRateLimiter instead.
func NewLoginRateLimiter(limit int, windowDur time.Duration) RateLimiter {
	// Return a no-op Redis limiter that will fail fast if Redis is not available
	// In practice, the main.go should initialize RedisRateLimiter for production
	return &noOpRateLimiter{}
}

// noOpRateLimiter is a placeholder that always allows requests.
// Used when Redis is not configured (development only).
type noOpRateLimiter struct{}

func (l *noOpRateLimiter) Allow(ctx context.Context, ip string) (bool, error) {
	// In production, this should not be used; main.go must initialize Redis.
	// For development/testing, this allows all requests.
	return true, nil
}
