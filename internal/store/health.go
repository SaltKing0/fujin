package store

import (
	"fmt"
	"time"
)

// HealthSample is a single HTTP health-check observation.
type HealthSample struct {
	Endpoint   string    `json:"endpoint"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int64     `json:"latency_ms"`
	CheckedAt  time.Time `json:"checked_at"`
}

// SaveHealthSample records one health-check result.
func (s *Store) SaveHealthSample(hs HealthSample) error {
	_, err := s.db.Exec(`
		INSERT INTO health_samples (endpoint, status_code, latency_ms, checked_at)
		VALUES (?, ?, ?, ?)`,
		hs.Endpoint, hs.StatusCode, hs.LatencyMs, hs.CheckedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("store: save health sample: %w", err)
	}
	return nil
}

// UptimeStats returns the uptime percentage (2xx/3xx responses / total) and
// the total number of samples for the given endpoint over the last N days.
// A nil endpoint aggregates across all endpoints.
func (s *Store) UptimeStats(endpoint string, days int) (uptime float64, totalSamples int, err error) {
	query := `
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN status_code >= 200 AND status_code < 400 THEN 1 ELSE 0 END), 0) AS ok
		FROM health_samples
		WHERE checked_at >= ?`
	args := []interface{}{time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)}
	if endpoint != "" {
		query += " AND endpoint = ?"
		args = append(args, endpoint)
	}

	var total, ok int
	if err := s.db.QueryRow(query, args...).Scan(&total, &ok); err != nil {
		return 0, 0, fmt.Errorf("store: uptime stats: %w", err)
	}
	if total == 0 {
		return 0, 0, nil
	}
	return float64(ok) / float64(total) * 100.0, total, nil
}
