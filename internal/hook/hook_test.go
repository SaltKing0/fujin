package hook

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SaltKing0/fujin/internal/config"
)

type fakeDecider struct {
	healthy map[string]bool
}

func (f fakeDecider) Healthy(r config.Remote) bool {
	return f.healthy[r.Name]
}

type fakeRunner struct {
	urls []string
}

func (f *fakeRunner) GitPush(url string, refs []string) (string, error) {
	f.urls = append(f.urls, url)
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

func TestParseRefs(t *testing.T) {
	input := "refs/heads/main 1111111111111111111111111111111111111111 refs/heads/main 0000000000000000000000000000000000000000\n" +
		"refs/tags/v1.0 2222222222222222222222222222222222222222 refs/tags/v1.0 0000000000000000000000000000000000000000\n"
	refs, err := ParseRefs(bufio.NewReader(strings.NewReader(input)))
	if err != nil {
		t.Fatalf("ParseRefs: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].LocalRef != "refs/heads/main" {
		t.Errorf("unexpected first ref: %+v", refs[0])
	}
	if refs[1].LocalRef != "refs/tags/v1.0" {
		t.Errorf("unexpected second ref: %+v", refs[1])
	}
}

func TestParseRefs_Empty(t *testing.T) {
	refs, err := ParseRefs(bufio.NewReader(strings.NewReader("")))
	if err != nil {
		t.Fatalf("ParseRefs: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs, got %d", len(refs))
	}
}

func TestPrePush_HealthyRemote_Passes(t *testing.T) {
	cfg := testCfg()
	runner := &fakeRunner{}
	code, msg := PrePush(cfg, fakeDecider{healthy: map[string]bool{"github": true, "gitea": false}}, runner, "origin", "git@github.com:u/r.git", []Ref{{LocalRef: "refs/heads/main"}})
	if code != 0 {
		t.Errorf("expected exit 0 when healthy, got %d (%s)", code, msg)
	}
	if len(runner.urls) != 0 {
		t.Errorf("expected no failover push, got %v", runner.urls)
	}
}

func TestPrePush_UnmatchedRemote_Passes(t *testing.T) {
	cfg := testCfg()
	runner := &fakeRunner{}
	// remote not in config — git handles it
	code, msg := PrePush(cfg, fakeDecider{healthy: map[string]bool{}}, runner, "bitbucket", "git@bitbucket.org:x/y.git", []Ref{{LocalRef: "refs/heads/main"}})
	if code != 0 {
		t.Errorf("expected exit 0 for unknown remote, got %d (%s)", code, msg)
	}
}

func TestPrePush_PrimaryDown_FailsOver(t *testing.T) {
	cfg := testCfg()
	runner := &fakeRunner{}
	code, msg := PrePush(cfg, fakeDecider{healthy: map[string]bool{"github": false, "gitea": true}}, runner, "origin", "git@github.com:u/r.git", []Ref{{LocalRef: "refs/heads/main"}})
	if code != 1 {
		t.Errorf("expected exit 1 (block original push), got %d", code)
	}
	if !strings.Contains(msg, "FAILOVER") {
		t.Errorf("expected FAILOVER in message, got: %s", msg)
	}
	if len(runner.urls) != 1 || runner.urls[0] != "git@gitea.example:u/r.git" {
		t.Errorf("expected failover push to gitea, got %v", runner.urls)
	}
}

func TestPrePush_AllDown_Blocks(t *testing.T) {
	cfg := testCfg()
	runner := &fakeRunner{}
	code, msg := PrePush(cfg, fakeDecider{healthy: map[string]bool{"github": false, "gitea": false}}, runner, "origin", "git@github.com:u/r.git", []Ref{{LocalRef: "refs/heads/main"}})
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(msg, "no failover") {
		t.Errorf("expected 'no failover' in message, got: %s", msg)
	}
	if len(runner.urls) != 0 {
		t.Errorf("expected no failover push, got %v", runner.urls)
	}
}

func TestInstall_Uninstall(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git", "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path, err := Install(repo)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !strings.HasSuffix(path, ".git/hooks/pre-push") {
		t.Errorf("unexpected path: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	if !strings.Contains(string(data), `hook pre-push "$@"`) {
		t.Errorf("hook does not call 'hook pre-push': %s", data)
	}
	// executable bit
	info, _ := os.Stat(path)
	if info.Mode().Perm()&0o111 == 0 {
		t.Error("hook is not executable")
	}

	if err := Uninstall(repo); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("hook still exists after uninstall")
	}
}

func TestUninstall_RefusesForeignHook(t *testing.T) {
	repo := t.TempDir()
	hooksDir := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	foreign := filepath.Join(hooksDir, "pre-push")
	if err := os.WriteFile(foreign, []byte("#!/bin/sh\n# someone else's hook\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := Uninstall(repo); err == nil {
		t.Fatal("expected error removing foreign hook")
	}
}

// --- raijin integration ---

// withFakeRaijin creates a fake raijin binary on PATH and returns a cleanup.
func withFakeRaijin(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	raijinPath := filepath.Join(dir, "raijin")
	if err := os.WriteFile(raijinPath, []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatalf("write fake raijin: %v", err)
	}
	os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Cleanup(func() { os.Setenv("PATH", os.Getenv("PATH")) })
}

func TestFailoverHint_NoRaijin(t *testing.T) {
	// remove raijin from PATH by pointing PATH at an empty dir
	os.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", os.Getenv("PATH"))

	hint := failoverHint()
	if hint != "" {
		t.Errorf("expected empty hint when raijin not on PATH, got %q", hint)
	}
}

func TestFailoverHint_NoWorkflows(t *testing.T) {
	withFakeRaijin(t)
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldDir)

	hint := failoverHint()
	if hint != "" {
		t.Errorf("expected empty hint when no workflows exist, got %q", hint)
	}
}

func TestFailoverHint_WithWorkflow(t *testing.T) {
	withFakeRaijin(t)
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.MkdirAll(".github/workflows", 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.WriteFile(".github/workflows/ci.yml", []byte("name: ci"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	hint := failoverHint()
	if !strings.Contains(hint, "raijin run") {
		t.Errorf("expected raijin run hint, got %q", hint)
	}
	if !strings.Contains(hint, "ci.yml") {
		t.Errorf("expected ci.yml in hint, got %q", hint)
	}
}
