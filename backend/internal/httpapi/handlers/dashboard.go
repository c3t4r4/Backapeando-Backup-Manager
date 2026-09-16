package handlers

import (
	"net/http"
	"time"

	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
)

// dashboardBackupWindowDays is the size of the rolling window used to
// compute the backup-runs KPIs (total/success/failed/success rate) on the
// dashboard summary.
const dashboardBackupWindowDays = 30

// dashboardAttentionListLimit caps how many servers/runs are returned in
// each attention-points list, so the endpoint stays cheap even with a large
// fleet or a long losing streak.
const dashboardAttentionListLimit = 10

// noDestinationLabel is the synthetic destination name used to group backup
// runs whose server has no storage_target_id configured.
const noDestinationLabel = "Sem destino"

// DashboardHandlers serves the read-only analytics summary consumed by the
// frontend's Dashboard screen (KPIs, charts, attention points). It never
// mutates state.
type DashboardHandlers struct {
	Servers        *repository.ServerRepo
	BackupRuns     *repository.BackupRunRepo
	StorageTargets *repository.StorageTargetRepo
}

type dashboardServerStatusCountsDTO struct {
	PendingKey            int `json:"pendingKey"`
	AwaitingAuthorization int `json:"awaitingAuthorization"`
	Ready                 int `json:"ready"`
	Disabled              int `json:"disabled"`
}

type dashboardServersDTO struct {
	Total               int                            `json:"total"`
	ByStatus            dashboardServerStatusCountsDTO `json:"byStatus"`
	WithConnectionError int                            `json:"withConnectionError"`
}

type dashboardBackupWindowDTO struct {
	Since              time.Time `json:"since"`
	Total              int       `json:"total"`
	Success            int       `json:"success"`
	Failed             int       `json:"failed"`
	SuccessRatePercent float64   `json:"successRatePercent"`
}

type dashboardAttentionDTO struct {
	ServersAwaitingAuthorization []serverDTO    `json:"serversAwaitingAuthorization"`
	ServersWithConnectionError   []serverDTO    `json:"serversWithConnectionError"`
	RecentFailures               []backupRunDTO `json:"recentFailures"`
}

type dashboardSummaryDTO struct {
	Servers              dashboardServersDTO      `json:"servers"`
	BackupRunsLast30Days dashboardBackupWindowDTO `json:"backupRunsLast30Days"`
	AttentionPoints      dashboardAttentionDTO    `json:"attentionPoints"`
}

// dashboardDestinationDTO is one entry of the destinations list shared by
// every series in dashboardBackupStatsDTO — the single source of truth for
// ordering, legend labels, and (client-side) color assignment across the 4
// per-destination charts. StorageTargetID is nil for the synthetic "Sem
// destino" entry.
type dashboardDestinationDTO struct {
	StorageTargetID *string `json:"storageTargetId"`
	Name            string  `json:"name"`
}

// dashboardDestinationSeriesDTO is one destination's data points within a
// dashboardBucketedStatsDTO. Data always has the same length as the
// enclosing Labels, zero-filled for buckets with no runs for this
// destination, so datasets never need to be aligned by searching.
type dashboardDestinationSeriesDTO struct {
	StorageTargetID *string `json:"storageTargetId"`
	Data            []int64 `json:"data"`
}

type dashboardBucketedStatsDTO struct {
	Since       *time.Time                      `json:"since,omitempty"`
	Until       *time.Time                      `json:"until,omitempty"`
	Year        *int                            `json:"year,omitempty"`
	Labels      []string                        `json:"labels"`
	CountSeries []dashboardDestinationSeriesDTO `json:"countSeries"`
	BytesSeries []dashboardDestinationSeriesDTO `json:"bytesSeries"`
}

// dashboardBackupStatsDTO is the response of GET /api/dashboard/backup-stats:
// per-destination backup counts and byte sums, bucketed daily over the last
// 30 days and monthly over the current year (RN-BACKUP-031).
type dashboardBackupStatsDTO struct {
	Destinations    []dashboardDestinationDTO `json:"destinations"`
	DailyLast30Days dashboardBucketedStatsDTO `json:"dailyLast30Days"`
	MonthlyThisYear dashboardBucketedStatsDTO `json:"monthlyThisYear"`
}

func toServerDTOs(servers []domain.Server) []serverDTO {
	out := make([]serverDTO, 0, len(servers))
	for _, s := range servers {
		out = append(out, toServerDTO(s))
	}
	return out
}

func toBackupRunDTOs(runs []domain.BackupRun) []backupRunDTO {
	out := make([]backupRunDTO, 0, len(runs))
	for _, run := range runs {
		out = append(out, toBackupRunDTO(run))
	}
	return out
}

// Summary aggregates server-status counts, a rolling backup-runs window,
// and attention points (servers stuck in awaiting_authorization, servers
// whose last connection test failed, and recent failed backup runs) into a
// single response for the dashboard's KPIs, charts, and attention section.
func (h *DashboardHandlers) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	statusCounts, err := h.Servers.CountByStatus(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	awaitingAuth, err := h.Servers.ListByStatus(ctx, domain.ServerStatusAwaitingAuthorization, dashboardAttentionListLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	withConnectionErrorTotal, err := h.Servers.CountWithConnectionError(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	withConnectionError, err := h.Servers.ListWithConnectionError(ctx, dashboardAttentionListLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	since := time.Now().AddDate(0, 0, -dashboardBackupWindowDays)
	runCounts, err := h.BackupRuns.CountByStatusSince(ctx, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	recentFailures, err := h.BackupRuns.RecentFailures(ctx, dashboardAttentionListLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	total := statusCounts[domain.ServerStatusPendingKey] + statusCounts[domain.ServerStatusAwaitingAuthorization] +
		statusCounts[domain.ServerStatusReady] + statusCounts[domain.ServerStatusDisabled]

	runTotal := runCounts[domain.BackupRunStatusQueued] + runCounts[domain.BackupRunStatusRunning] +
		runCounts[domain.BackupRunStatusSuccess] + runCounts[domain.BackupRunStatusFailed]

	// Success rate only makes sense over completed runs — queued/running
	// are still in flight and would otherwise dilute the percentage.
	completed := runCounts[domain.BackupRunStatusSuccess] + runCounts[domain.BackupRunStatusFailed]
	var successRate float64
	if completed > 0 {
		successRate = float64(runCounts[domain.BackupRunStatusSuccess]) / float64(completed) * 100
	}

	writeJSON(w, http.StatusOK, dashboardSummaryDTO{
		Servers: dashboardServersDTO{
			Total: total,
			ByStatus: dashboardServerStatusCountsDTO{
				PendingKey:            statusCounts[domain.ServerStatusPendingKey],
				AwaitingAuthorization: statusCounts[domain.ServerStatusAwaitingAuthorization],
				Ready:                 statusCounts[domain.ServerStatusReady],
				Disabled:              statusCounts[domain.ServerStatusDisabled],
			},
			WithConnectionError: withConnectionErrorTotal,
		},
		BackupRunsLast30Days: dashboardBackupWindowDTO{
			Since:              since,
			Total:              runTotal,
			Success:            runCounts[domain.BackupRunStatusSuccess],
			Failed:             runCounts[domain.BackupRunStatusFailed],
			SuccessRatePercent: successRate,
		},
		AttentionPoints: dashboardAttentionDTO{
			ServersAwaitingAuthorization: toServerDTOs(awaitingAuth),
			ServersWithConnectionError:   toServerDTOs(withConnectionError),
			RecentFailures:               toBackupRunDTOs(recentFailures),
		},
	})
}

// BackupStats aggregates backup-run counts and byte sums by destination
// (the server's CURRENT storage target — RN-BACKUP-031), bucketed daily over
// the last 30 days and monthly over the current year (Jan–Dec, future
// months zero-filled), feeding the dashboard's 4 stacked bar charts.
func (h *DashboardHandlers) BackupStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	targets, err := h.StorageTargets.List(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	destinations := make([]dashboardDestinationDTO, 0, len(targets)+1)
	for _, t := range targets {
		id := t.ID
		destinations = append(destinations, dashboardDestinationDTO{StorageTargetID: &id, Name: t.Name})
	}
	destinations = append(destinations, dashboardDestinationDTO{StorageTargetID: nil, Name: noDestinationLabel})

	now := time.Now().UTC()

	dayLabels := make([]string, 0, dashboardBackupWindowDays)
	until := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	since := until.AddDate(0, 0, -dashboardBackupWindowDays)
	for d := since; d.Before(until); d = d.AddDate(0, 0, 1) {
		dayLabels = append(dayLabels, d.Format("2006-01-02"))
	}

	dailyBuckets, err := h.BackupRuns.CountAndBytesByDestination(ctx, "day", since, until)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	year := now.Year()
	sinceYear := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	untilYear := sinceYear.AddDate(1, 0, 0)

	monthLabels := make([]string, 0, 12)
	for m := sinceYear; m.Before(untilYear); m = m.AddDate(0, 1, 0) {
		monthLabels = append(monthLabels, m.Format("2006-01"))
	}

	monthlyBuckets, err := h.BackupRuns.CountAndBytesByDestination(ctx, "month", sinceYear, untilYear)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	dailyCount, dailyBytes := pivotDestinationBuckets(dailyBuckets, dayLabels, "2006-01-02", destinations)
	monthlyCount, monthlyBytes := pivotDestinationBuckets(monthlyBuckets, monthLabels, "2006-01", destinations)

	writeJSON(w, http.StatusOK, dashboardBackupStatsDTO{
		Destinations: destinations,
		DailyLast30Days: dashboardBucketedStatsDTO{
			Since:       &since,
			Until:       &until,
			Labels:      dayLabels,
			CountSeries: dailyCount,
			BytesSeries: dailyBytes,
		},
		MonthlyThisYear: dashboardBucketedStatsDTO{
			Year:        &year,
			Labels:      monthLabels,
			CountSeries: monthlyCount,
			BytesSeries: monthlyBytes,
		},
	})
}

// pivotDestinationBuckets turns flat (bucket, destination) rows into one
// count series and one bytes series per destination, each aligned
// position-for-position with labels and zero-filled where a destination had
// no runs in a given bucket. layout formats each bucket's time.Time the same
// way labels were formatted, so buckets can be matched back to their label
// position. A bucket whose destination isn't in destinations (shouldn't
// happen, since the query resolves it from the same tables) falls back to
// the "Sem destino" entry rather than being silently dropped.
func pivotDestinationBuckets(buckets []domain.BackupRunDestinationBucket, labels []string, layout string, destinations []dashboardDestinationDTO) (countSeries, bytesSeries []dashboardDestinationSeriesDTO) {
	labelIndex := make(map[string]int, len(labels))
	for i, l := range labels {
		labelIndex[l] = i
	}

	destKey := func(id *string) string {
		if id == nil {
			return ""
		}
		return *id
	}

	destIndex := make(map[string]int, len(destinations))
	countSeries = make([]dashboardDestinationSeriesDTO, len(destinations))
	bytesSeries = make([]dashboardDestinationSeriesDTO, len(destinations))
	for i, d := range destinations {
		destIndex[destKey(d.StorageTargetID)] = i
		countSeries[i] = dashboardDestinationSeriesDTO{StorageTargetID: d.StorageTargetID, Data: make([]int64, len(labels))}
		bytesSeries[i] = dashboardDestinationSeriesDTO{StorageTargetID: d.StorageTargetID, Data: make([]int64, len(labels))}
	}

	noDestinationIdx := len(destinations) - 1
	for _, b := range buckets {
		labelPos, ok := labelIndex[b.Bucket.Format(layout)]
		if !ok {
			continue
		}
		destPos, ok := destIndex[destKey(b.StorageTargetID)]
		if !ok {
			destPos = noDestinationIdx
		}
		countSeries[destPos].Data[labelPos] += int64(b.RunCount)
		bytesSeries[destPos].Data[labelPos] += b.TotalBytes
	}
	return countSeries, bytesSeries
}
