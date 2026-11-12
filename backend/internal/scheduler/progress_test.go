package scheduler

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestByteCounter_Accumulates(t *testing.T) {
	c := &byteCounter{r: strings.NewReader("abcdef")}
	buf := make([]byte, 2)
	var got []byte
	for {
		n, err := c.Read(buf)
		got = append(got, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read: %v", err)
		}
	}
	if string(got) != "abcdef" || c.Bytes() != 6 {
		t.Fatalf("got %q bytes=%d", got, c.Bytes())
	}
}

func TestProgressInterval(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("BACKUP_PROGRESS_INTERVAL", "")
		if progressInterval() != 15*time.Second {
			t.Fatalf("default = %s", progressInterval())
		}
	})
	t.Run("override", func(t *testing.T) {
		t.Setenv("BACKUP_PROGRESS_INTERVAL", "2")
		if progressInterval() != 2*time.Second {
			t.Fatalf("override = %s", progressInterval())
		}
	})
	t.Run("zero disables", func(t *testing.T) {
		t.Setenv("BACKUP_PROGRESS_INTERVAL", "0")
		if progressInterval() != 0 {
			t.Fatalf("zero = %s", progressInterval())
		}
	})
	t.Run("invalid", func(t *testing.T) {
		t.Setenv("BACKUP_PROGRESS_INTERVAL", "nope")
		if progressInterval() != 15*time.Second {
			t.Fatalf("invalid = %s", progressInterval())
		}
	})
}

func TestWatchProgress_LogsZeroBytesWhileIdle(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	counter := &byteCounter{r: strings.NewReader("")}
	stop := watchProgress(ctx, logger, 20*time.Millisecond, "dump_upload", "run-1", counter.Bytes, time.Now())
	defer stop()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		out := buf.String()
		if strings.Contains(out, "dump_upload") && strings.Contains(out, "bytesRead=0") && strings.Contains(out, "run-1") {
			stop()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("heartbeat missing: %s", buf.String())
}

func TestWatchProgress_DisabledIsNoop(t *testing.T) {
	stop := watchProgress(context.Background(), slog.Default(), 0, "dump_upload", "run-1", func() int64 { return 0 }, time.Now())
	stop()
	stop()
}
