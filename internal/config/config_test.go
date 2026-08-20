package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoad_Basic(t *testing.T) {
	t.Setenv("FUJIN_PRIMARY_URL", "")
	t.Setenv("FUJIN_DB_PATH", "")
	path := writeConfig(t, `
primary:
  name: github
  url: git@github.com:user/repo.git
failover:
  - name: gitea
    url: git@gitea.example.com:user/repo.git
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Primary.Name != "github" || cfg.Primary.URL != "git@github.com:user/repo.git" {
		t.Errorf("unexpected primary: %+v", cfg.Primary)
	}
	if len(cfg.Failover) != 1 || cfg.Failover[0].Name != "gitea" {
		t.Errorf("unexpected failover: %+v", cfg.Failover)
	}
	if !cfg.Health.Statuspage {
		t.Error("expected statuspage health enabled by default")
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("FUJIN_PRIMARY_URL", "git@github.com:env/repo.git")
	path := writeConfig(t, `
primary:
  name: github
  url: git@github.com:file/repo.git
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Primary.URL != "git@github.com:env/repo.git" {
		t.Errorf("expected env override, got %q", cfg.Primary.URL)
	}
}

func TestLoad_MissingPrimary(t *testing.T) {
	t.Setenv("FUJIN_PRIMARY_URL", "")
	path := writeConfig(t, "failover:\n  - name: gitea\n    url: git@gitea.example.com:r.git\n")
	if _, err := Load(path); err == nil {
		t.Fatal("expected error when primary missing")
	}
}

func TestLoad_MissingFile_UsesDefaults(t *testing.T) {
	t.Setenv("FUJIN_PRIMARY_URL", "git@github.com:x/y.git")
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Health.Endpoints) != 2 {
		t.Errorf("expected 2 default endpoints, got %d", len(cfg.Health.Endpoints))
	}
}

func TestAllRemotes(t *testing.T) {
	cfg := Config{
		Primary:  Remote{Name: "github", URL: "u1"},
		Failover: []Remote{{Name: "gitea", URL: "u2"}, {Name: "gitlab", URL: "u3"}},
	}
	all := cfg.AllRemotes()
	if len(all) != 3 {
		t.Fatalf("expected 3 remotes, got %d", len(all))
	}
	if all[0].Name != "github" || all[2].Name != "gitlab" {
		t.Errorf("unexpected order: %+v", all)
	}
}
