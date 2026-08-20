package store

import (
	"fmt"
	"time"
)

// PendingPush is a push queued because all remotes were unhealthy.
type PendingPush struct {
	ID        int64
	Refspec   string
	Status    string // "pending" | "ok" | "failed"
	CreatedAt time.Time
	PushedAt  *time.Time
}

// EnqueuePush adds a refspec to the offline queue.
func (s *Store) EnqueuePush(refspec string) error {
	_, err := s.db.Exec(`
		INSERT INTO pending_pushes (refspec, status, created_at)
		VALUES (?, 'pending', ?)`,
		refspec, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("store: enqueue push: %w", err)
	}
	return nil
}

// PendingPushes returns all queued pushes that still need delivery
// (pending or previously failed attempts — both are retried).
func (s *Store) PendingPushes() ([]PendingPush, error) {
	rows, err := s.db.Query(`
		SELECT id, refspec, status, created_at, pushed_at
		FROM pending_pushes
		WHERE status IN ('pending', 'failed')
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("store: query pending pushes: %w", err)
	}
	defer rows.Close()

	var out []PendingPush
	for rows.Next() {
		var p PendingPush
		var createdRaw string
		var pushedRaw *string
		if err := rows.Scan(&p.ID, &p.Refspec, &p.Status, &createdRaw, &pushedRaw); err != nil {
			return nil, fmt.Errorf("store: scan pending push: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, createdRaw); err == nil {
			p.CreatedAt = t
		}
		if pushedRaw != nil {
			if t, err := time.Parse(time.RFC3339, *pushedRaw); err == nil {
				p.PushedAt = &t
			}
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MarkPushResult updates a queued push after a flush attempt.
func (s *Store) MarkPushResult(id int64, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		UPDATE pending_pushes SET status = ?, pushed_at = ?
		WHERE id = ?`, status, now, id)
	if err != nil {
		return fmt.Errorf("store: mark push %d: %w", id, err)
	}
	return nil
}
