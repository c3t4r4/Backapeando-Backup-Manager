package scheduler

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// NextRunTime parses a standard 5-field cron expression and returns the next
// time it fires strictly after from. Shared by claimAndEnqueue (recurring
// reschedule after a claim) and the test-connection handler (first-time
// bootstrap of a server's next_run_at) so the two writers of next_run_at
// can never diverge on how "next run" is computed.
func NextRunTime(cronExpression string, from time.Time) (time.Time, error) {
	cronExpr, err := cron.ParseStandard(cronExpression)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cron expression %q: %w", cronExpression, err)
	}
	return cronExpr.Next(from), nil
}
