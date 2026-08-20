package push

import (
	"errors"
	"strings"
	"testing"

	"github.com/SaltKing0/fujin/internal/config"
	"github.com/SaltKing0/fujin/internal/store"
)

type fakeDecider struct {
	healthy map[string]bool
}

func (f fakeDecider) Healthy(r config.Remote) bool {
	return f.healthy[r.Name]
}

type fakeRunner struct {
	calls    []string // urls pushed
	failURLs map[string]bool
}

func (f *fakeRunner) GitPush(url string, refspecs []string) (string, error) {
	f.calls = append(f.calls, url)
	if f.failURLs[url] {
		return "remote error", errors.New("push rejected")
	}
	return "ok", nil
}

func testCfg() *config.Config {
	return &config.Config{
		Primary: config.Remote{Name: "github", URL: "git@github.com:u/r.git"},
		Failover: []config.Remote{
			{Name: "gitea", URL: "git@gitea.example:u/r.git"},
		},
	}
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPush_PrimaryHealthy(t *testing.T) {
	runner := &fakeRunner{}
	p := New(testCfg(), nil, fakeDecider{healthy: map[string]bool{"github": true, "gitea": false}})
	p.Runner = runner

	res := p.Push([]string{"main"})
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if res.Failover {
		t.Error("expected primary push, got failover")
	}
	if len(runner.calls) != 1 || runner.calls[0] != "git@github.com:u/r.git" {
		t.Errorf("expected push to github, got %v", runner.calls)
	}
}

func TestPush_FailoverWhenPrimaryDown(t *testing.T) {
	runner := &fakeRunner{}
	p := New(testCfg(), nil, fakeDecider{healthy: map[string]bool{"github": false, "gitea": true}})
	p.Runner = runner

	res := p.Push([]string{"main"})
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if !res.Failover {
		t.Error("expected failover push")
	}
	if res.Remote.Name != "gitea" {
		t.Errorf("expected push to gitea, got %s", res.Remote.Name)
	}
	if len(runner.calls) != 1 {
		t.Errorf("expected exactly 1 push call, got %d", len(runner.calls))
	}
}

func TestPush_AllDown(t *testing.T) {
	runner := &fakeRunner{}
	p := New(testCfg(), nil, fakeDecider{healthy: map[string]bool{"github": false, "gitea": false}})
	p.Runner = runner

	res := p.Push([]string{"main"})
	if res.Err == nil {
		t.Fatal("expected error when all remotes down")
	}
	if !strings.Contains(res.Err.Error(), "all remotes unhealthy") {
		t.Errorf("unexpected error: %v", res.Err)
	}
	if len(runner.calls) != 0 {
		t.Errorf("expected no push calls, got %d", len(runner.calls))
	}
}

func TestPush_RecordsHistory(t *testing.T) {
	st := openStore(t)
	runner := &fakeRunner{}
	p := New(testCfg(), st, fakeDecider{healthy: map[string]bool{"github": false, "gitea": true}})
	p.Runner = runner

	if res := p.Push([]string{"main"}); res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}

	records, err := st.RecentPushes(5)
	if err != nil {
		t.Fatalf("RecentPushes: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Remote != "gitea" || !records[0].Failover || records[0].Status != "ok" {
		t.Errorf("unexpected record: %+v", records[0])
	}
}

func TestPush_PushErrorRecorded(t *testing.T) {
	st := openStore(t)
	runner := &fakeRunner{failURLs: map[string]bool{"git@github.com:u/r.git": true}}
	p := New(testCfg(), st, fakeDecider{healthy: map[string]bool{"github": true, "gitea": false}})
	p.Runner = runner

	res := p.Push([]string{"main"})
	if res.Err == nil {
		t.Fatal("expected push error")
	}
	records, _ := st.RecentPushes(5)
	if len(records) != 1 || records[0].Status != "failed" {
		t.Errorf("expected failed record, got %+v", records)
	}
}
