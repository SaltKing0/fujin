package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSavePush_AndRecent(t *testing.T) {
	s := openTestStore(t)

	now := time.Date(2026, 8, 20, 22, 30, 0, 0, time.UTC)
	p1 := PushRecord{Remote: "github", Refspec: "main", Status: "ok", PushedAt: now, Failover: false}
	p2 := PushRecord{Remote: "gitea", Refspec: "main", Status: "ok", PushedAt: now.Add(time.Minute), Failover: true}

	if err := s.SavePush(p1); err != nil {
		t.Fatalf("SavePush p1: %v", err)
	}
	if err := s.SavePush(p2); err != nil {
		t.Fatalf("SavePush p2: %v", err)
	}

	recs, err := s.RecentPushes(10)
	if err != nil {
		t.Fatalf("RecentPushes: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
	// newest first
	if recs[0].Remote != "gitea" || !recs[0].Failover {
		t.Errorf("expected gitea failover first, got %+v", recs[0])
	}
	if recs[1].Remote != "github" || recs[1].Failover {
		t.Errorf("expected github primary second, got %+v", recs[1])
	}
}

func TestSavePush_Failed(t *testing.T) {
	s := openTestStore(t)
	if err := s.SavePush(PushRecord{Remote: "github", Refspec: "main", Status: "failed", ErrMsg: "rejected", PushedAt: time.Now(), Failover: false}); err != nil {
		t.Fatalf("SavePush: %v", err)
	}
	recs, _ := s.RecentPushes(5)
	if len(recs) != 1 || recs[0].Status != "failed" || recs[0].ErrMsg != "rejected" {
		t.Errorf("unexpected record: %+v", recs)
	}
}

func TestRecentPushes_Limit(t *testing.T) {
	s := openTestStore(t)
	for i := 0; i < 5; i++ {
		_ = s.SavePush(PushRecord{Remote: "github", Refspec: "main", Status: "ok", PushedAt: time.Now().Add(time.Duration(i) * time.Minute)})
	}
	recs, _ := s.RecentPushes(3)
	if len(recs) != 3 {
		t.Errorf("expected 3 records with limit 3, got %d", len(recs))
	}
}
