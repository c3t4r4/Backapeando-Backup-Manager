package scheduler

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

// byteCounter counts bytes read from an underlying reader. Read blocks
// exactly like the wrapped reader, so a heartbeat can report 0 while a
// dump or upload is stuck before the first byte.
type byteCounter struct {
	r io.Reader
	n atomic.Int64
}

func (b *byteCounter) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	if n > 0 {
		b.n.Add(int64(n))
	}
	return n, err
}

func (b *byteCounter) Bytes() int64 {
	return b.n.Load()
}

// ProgressInterval is how often a blocked backup reports that it is still
// running. BACKUP_PROGRESS_INTERVAL is seconds. 0 disables the heartbeat.
// An empty or invalid value uses 15s.
func ProgressInterval() time.Duration {
	return progressInterval()
}

func progressInterval() time.Duration {
	raw := os.Getenv("BACKUP_PROGRESS_INTERVAL")
	if raw == "" {
		return 15 * time.Second
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs < 0 {
		return 15 * time.Second
	}
	return time.Duration(secs) * time.Second
}

// watchProgress logs stage, bytes and elapsed time until stop is called or
// ctx ends. every <= 0 returns a no-op stop. stop is idempotent.
func watchProgress(ctx context.Context, logger *slog.Logger, every time.Duration, stage, runID string, bytes func() int64, started time.Time) (stop func()) {
	if every <= 0 || logger == nil {
		return func() {}
	}
	done := make(chan struct{})
	var stopped atomic.Bool
	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				logger.InfoContext(ctx, "backup still in progress",
					slog.String("stage", stage),
					slog.String("backupRunId", runID),
					slog.Int64("bytesRead", bytes()),
					slog.Int64("elapsedMs", time.Since(started).Milliseconds()),
				)
			}
		}
	}()
	return func() {
		if stopped.CompareAndSwap(false, true) {
			close(done)
		}
	}
}
