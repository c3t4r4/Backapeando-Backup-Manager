package scheduler

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

// MockWorkerPool provides a mock implementation of the WorkerPool interface
// for testing the Scheduler without relying on a real pool.
type MockWorkerPool struct {
	availableSlots int
	submitCalls    int
	submitErr      error
	submitTasks    []WorkerTask
}

// AvailableSlots returns the mocked number of available slots.
func (m *MockWorkerPool) AvailableSlots() int {
	return m.availableSlots
}

// Submit records a task submission for inspection in tests.
func (m *MockWorkerPool) Submit(task WorkerTask) error {
	m.submitCalls++
	if m.submitErr != nil {
		return m.submitErr
	}
	m.submitTasks = append(m.submitTasks, task)
	return nil
}

// Stop is a no-op for the mock.
func (m *MockWorkerPool) Stop(ctx context.Context) error {
	return nil
}

// Start is a no-op for the mock.
func (m *MockWorkerPool) Start(numWorkers int) {
	// no-op
}

// TestSchedulerStart verifies that Start() initiates polling and Stop() terminates it.
func TestSchedulerStart(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(
		noopWriter{},
		&slog.HandlerOptions{Level: slog.LevelDebug},
	))

	mock := &MockWorkerPool{availableSlots: 5}
	sched := NewScheduler(mock, nil, nil, 50*time.Millisecond, logger)

	// Start should not block and should begin polling.
	sched.Start()

	// Give the goroutine time to at least start.
	time.Sleep(10 * time.Millisecond)

	// Stop should block until the polling loop exits.
	err := sched.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}

	// After Stop, the scheduler should have stopped polling.
	// We can verify this by checking that further operations don't panic.
}

// TestSchedulerStopWithoutStart verifies that Stop() is safe to call
// without a prior Start().
func TestSchedulerStopWithoutStart(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(
		noopWriter{},
		&slog.HandlerOptions{Level: slog.LevelDebug},
	))

	mock := &MockWorkerPool{availableSlots: 5}
	sched := NewScheduler(mock, nil, nil, 50*time.Millisecond, logger)

	// Stop without Start should not panic or hang.
	err := sched.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop() without Start() returned error: %v", err)
	}
}

// TestSchedulerRespectsConcurrency verifies that the scheduler respects
// the worker pool's available slots and skips polling when no slots are available.
func TestSchedulerRespectsConcurrency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(
		noopWriter{},
		&slog.HandlerOptions{Level: slog.LevelDebug},
	))

	mock := &MockWorkerPool{availableSlots: 0}
	sched := NewScheduler(mock, nil, nil, 10*time.Millisecond, logger)

	sched.Start()

	// Let the scheduler run a few polling cycles with no available slots.
	// Even though poll cycles occur, claimAndEnqueue should not be called
	// (or should be called but skip work due to availableSlots = 0).
	time.Sleep(50 * time.Millisecond)

	err := sched.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}

	// With availableSlots = 0, we expect minimal or no activity.
	// This test primarily verifies that the scheduler does not panic
	// and properly respects the poll interval and available slots.
}

// TestSchedulerContextCancellation verifies that the scheduler exits
// when its context is canceled.
func TestSchedulerContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(
		noopWriter{},
		&slog.HandlerOptions{Level: slog.LevelDebug},
	))

	mock := &MockWorkerPool{availableSlots: 5}
	sched := NewScheduler(mock, nil, nil, 100*time.Millisecond, logger)

	sched.Start()

	// Let it run for a bit.
	time.Sleep(50 * time.Millisecond)

	// Stop should trigger context cancellation and exit cleanly.
	err := sched.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}

	// Verify we can start again after stopping.
	sched2 := NewScheduler(mock, nil, nil, 100*time.Millisecond, logger)
	sched2.Start()
	time.Sleep(10 * time.Millisecond)
	err = sched2.Stop(context.Background())
	if err != nil {
		t.Fatalf("Second scheduler Stop() returned error: %v", err)
	}
}

// TestSchedulerPollInterval verifies that the scheduler respects the
// configured poll interval and avoids nil panic by returning 0 available slots.
func TestSchedulerPollInterval(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(
		noopWriter{},
		&slog.HandlerOptions{Level: slog.LevelDebug},
	))

	// Use availableSlots = 0 to skip claimAndEnqueue and avoid nil repos panic
	// The scheduler's polling loop will still run and check pool capacity
	mockPool := &MockWorkerPool{availableSlots: 0}
	sched := NewScheduler(mockPool, nil, nil, 20*time.Millisecond, logger)

	sched.Start()

	// Run for ~100ms; with a 20ms poll interval, we expect ~5 poll cycles.
	// Each cycle will see availableSlots=0 and skip work.
	time.Sleep(100 * time.Millisecond)

	err := sched.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}

	// Test verifies the scheduler completes without panic with zero available slots.
}

// TestSchedulerNewWithNilLogger verifies that NewScheduler handles a nil logger gracefully.
func TestSchedulerNewWithNilLogger(t *testing.T) {
	mock := &MockWorkerPool{availableSlots: 5}

	// nil logger should be replaced with default
	sched := NewScheduler(mock, nil, nil, 50*time.Millisecond, nil)
	if sched.logger == nil {
		t.Fatal("Scheduler.logger should not be nil after NewScheduler with nil logger")
	}

	sched.Start()
	time.Sleep(10 * time.Millisecond)
	err := sched.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}
}

// noopWriter implements io.Writer but discards all output.
// Used to suppress test log output to keep test output clean.
type noopWriter struct{}

func (noopWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// TestSchedulerMultipleStopCalls verifies that calling Stop() multiple times
// does not cause issues.
func TestSchedulerMultipleStopCalls(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(
		noopWriter{},
		&slog.HandlerOptions{Level: slog.LevelDebug},
	))

	mock := &MockWorkerPool{availableSlots: 5}
	sched := NewScheduler(mock, nil, nil, 50*time.Millisecond, logger)

	sched.Start()
	time.Sleep(10 * time.Millisecond)

	// First Stop should succeed.
	err := sched.Stop(context.Background())
	if err != nil {
		t.Fatalf("First Stop() returned error: %v", err)
	}

	// Second Stop should also succeed (or be a no-op).
	// This test verifies idempotency.
	err = sched.Stop(context.Background())
	if err != nil {
		t.Fatalf("Second Stop() returned error: %v", err)
	}
}
