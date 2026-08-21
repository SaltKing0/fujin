package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit_WritesConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	// simulate user input: primary url, name defaults, failover url+name, endpoints default
	input := "git@github.com:user/repo.git\n\ngit@gitea.example.com:user/repo.git\n\n\n"
	r := strings.NewReader(input)
	var out strings.Builder

	written, err := Init(r, &out, cfgPath)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if written != cfgPath {
		t.Errorf("expected %s, got %s", cfgPath, written)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"git@github.com:user/repo.git",
		"git@gitea.example.com:user/repo.git",
		"name: github",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("config missing %q:\n%s", want, content)
		}
	}
}

func TestInit_EmptyPrimaryFails(t *testing.T) {
	t.Chdir(t.TempDir()) // no origin remote here
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	r := strings.NewReader("\n") // empty primary URL
	var out strings.Builder

	if _, err := Init(r, &out, cfgPath); err == nil {
		t.Fatal("expected error when primary URL is empty")
	}
}

func TestInit_NoFailover(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	// primary url, name default, empty failover url
	r := strings.NewReader("git@github.com:user/repo.git\n\n\n")
	var out strings.Builder

	if _, err := Init(r, &out, cfgPath); err != nil {
		t.Fatalf("Init: %v", err)
	}
	data, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(data), "failover") && strings.Contains(string(data), "gitea.example") {
		t.Errorf("expected no failover remote in config:\n%s", data)
	}
}
