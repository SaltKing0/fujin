package store

import (
	"fmt"
	"time"
)

// PushRecord is one fujin push attempt.
type PushRecord struct {
	ID       int64
	Remote   string // which remote the push went to
	Refspec  string // the refspec(s) pushed
	Status   string // "ok" | "failed"
	ErrMsg   string
	PushedAt time.Time
	Failover bool // true if pushed to a failover remote
}

// SavePush records a push attempt.
func (s *Store) SavePush(p PushRecord) error {
	_, err := s.db.Exec(`
		INSERT INTO push_history (remote, refspec, status, err_msg, pushed_at, failover)
		VALUES (?, ?, ?, ?, ?, ?)`,
		p.Remote, p.Refspec, p.Status, p.ErrMsg, p.PushedAt.UTC().Format(time.RFC3339), p.Failover)
	if err != nil {
		return fmt.Errorf("store: save push: %w", err)
	}
	return nil
}

// RecentPushes returns the last N push records, newest first.
func (s *Store) RecentPushes(limit int) ([]PushRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, remote, refspec, status, err_msg, pushed_at, failover
		FROM push_history
		ORDER BY pushed_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: query pushes: %w", err)
	}
	defer rows.Close()

	var out []PushRecord
	for rows.Next() {
		var p PushRecord
		var pushedRaw string
		if err := rows.Scan(&p.ID, &p.Remote, &p.Refspec, &p.Status, &p.ErrMsg, &pushedRaw, &p.Failover); err != nil {
			return nil, fmt.Errorf("store: scan push: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, pushedRaw); err == nil {
			p.PushedAt = t
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
