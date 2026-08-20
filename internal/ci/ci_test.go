package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeWorkflowDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return dir
}

func TestFindWorkflows(t *testing.T) {
	dir := makeWorkflowDir(t)
	for _, f := range []string{"ci.yml", "deploy.yaml", "notes.txt", "ci.yml.example"} {
		_ = os.WriteFile(filepath.Join(dir, ".github", "workflows", f), []byte("x"), 0o644)
	}
	files, err := FindWorkflows(dir)
	if err != nil {
		t.Fatalf("FindWorkflows: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 workflow files, got %v", files)
	}
	if !strings.HasSuffix(files[0], "ci.yml") || !strings.HasSuffix(files[1], "deploy.yaml") {
		t.Errorf("unexpected files: %v", files)
	}
}

func TestFindWorkflows_None(t *testing.T) {
	dir := t.TempDir()
	files, err := FindWorkflows(dir)
	if err != nil {
		t.Fatalf("FindWorkflows: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected none, got %v", files)
	}
}

type fakeRunner struct {
	results map[string]bool
	calls   []string
}

func (f *fakeRunner) Run(dir, workflow string, report bool) (bool, string) {
	f.calls = append(f.calls, workflow)
	ok := f.results[workflow]
	if ok {
		return true, "[ok] " + workflow + "\n"
	}
	return false, "[fail] " + workflow + "\n"
}

func TestRunAll_AllPass(t *testing.T) {
	dir := makeWorkflowDir(t)
	_ = os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yml"), []byte("x"), 0o644)

	// raijin must be on PATH for RunAll's pre-check
	binDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(binDir, "raijin"), []byte("#!/bin/sh\nexit 0"), 0o755)
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	defer os.Setenv("PATH", os.Getenv("PATH"))

	r := &fakeRunner{results: map[string]bool{filepath.Join(dir, ".github", "workflows", "ci.yml"): true}}
	d := Dispatcher{Runner: r}
	failed, err := d.RunAll(dir)
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failures, got %v", failed)
	}
	if len(r.calls) != 1 || r.calls[0] != filepath.Join(dir, ".github", "workflows", "ci.yml") {
		t.Errorf("unexpected calls: %v", r.calls)
	}
}

func TestRunAll_OneFails(t *testing.T) {
	dir := makeWorkflowDir(t)
	_ = os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yml"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".github", "workflows", "deploy.yaml"), []byte("x"), 0o644)

	binDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(binDir, "raijin"), []byte("#!/bin/sh\nexit 0"), 0o755)
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	defer os.Setenv("PATH", os.Getenv("PATH"))

	r := &fakeRunner{results: map[string]bool{
		filepath.Join(dir, ".github", "workflows", "ci.yml"):      true,
		filepath.Join(dir, ".github", "workflows", "deploy.yaml"): false,
	}}
	d := Dispatcher{Runner: r}
	failed, err := d.RunAll(dir)
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if len(failed) != 1 {
		t.Errorf("expected 1 failure, got %v", failed)
	}
	if msg := FormatFailed(failed); !strings.Contains(msg, "deploy.yaml") {
		t.Errorf("unexpected failure message: %q", msg)
	}
}

func TestRunAll_NoWorkflows(t *testing.T) {
	d := Dispatcher{Runner: &fakeRunner{}}
	if _, err := d.RunAll(t.TempDir()); err == nil {
		t.Fatal("expected error when no workflows exist")
	}
}

func TestRunAll_RaijinMissing(t *testing.T) {
	dir := makeWorkflowDir(t)
	_ = os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yml"), []byte("x"), 0o644)

	os.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", os.Getenv("PATH"))

	d := Dispatcher{Runner: &fakeRunner{}}
	if _, err := d.RunAll(dir); err == nil {
		t.Fatal("expected error when raijin missing")
	}
}

func TestHasWorkflows(t *testing.T) {
	dir := makeWorkflowDir(t)
	if HasWorkflows(dir) {
		t.Error("expected false without workflows")
	}
	_ = os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yml"), []byte("x"), 0o644)
	if !HasWorkflows(dir) {
		t.Error("expected true with workflow")
	}
}
