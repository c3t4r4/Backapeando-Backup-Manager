package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	brazilTZ  *time.Location
	tzMutex   sync.Once
	tzLoadErr error
)

// loadBrazilTZ lazily loads the America/Sao_Paulo timezone, caching it for
// subsequent calls. This ensures that all cron expressions are consistently
// interpreted in Brazil's local time, not UTC.
func loadBrazilTZ() (*time.Location, error) {
	tzMutex.Do(func() {
		brazilTZ, tzLoadErr = time.LoadLocation("America/Sao_Paulo")
	})
	return brazilTZ, tzLoadErr
}

// NextRunTime parses a standard 5-field cron expression and returns the next
// time it fires strictly after from, interpreted in America/Sao_Paulo timezone.
// This ensures consistent behavior across all environments (dev, prod, containers,
// local machines) regardless of the process's system timezone or TZ variable.
// Shared by claimAndEnqueue (recurring reschedule after a claim) and the
// test-connection handler (first-time bootstrap of a server's next_run_at) so
// the two writers of next_run_at can never diverge on how "next run" is computed.
func NextRunTime(cronExpression string, from time.Time) (time.Time, error) {
	loc, err := loadBrazilTZ()
	if err != nil {
		return time.Time{}, fmt.Errorf("load timezone: %w", err)
	}

	cronExpr, err := cron.ParseStandard(cronExpression)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cron expression %q: %w", cronExpression, err)
	}

	// Convert the input time to Brazil timezone before computing next run.
	// This ensures "0 3 * * *" means "3am on America/Sao_Paulo", not "3am
	// in whatever zone from already carries".
	fromInBrazil := from.In(loc)
	nextInBrazil := cronExpr.Next(fromInBrazil)

	return nextInBrazil, nil
}
