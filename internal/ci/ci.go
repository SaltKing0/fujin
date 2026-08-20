package ci

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// FindWorkflows returns the workflow files in .github/workflows/
// (*.yml and *.yaml), sorted.
func FindWorkflows(dir string) ([]string, error) {
	var files []string
	for _, pattern := range []string{".github/workflows/*.yml", ".github/workflows/*.yaml"} {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, fmt.Errorf("ci: glob %s: %w", pattern, err)
		}
		files = append(files, matches...)
	}
	sort.Strings(files)
	return files, nil
}

// Runner executes raijin for one workflow file. It is satisfied by RunRaijin
// and mocked in tests.
type Runner interface {
	// Run executes `raijin run <workflow> [--report]` in dir and returns
	// (success, output).
	Run(dir, workflow string, report bool) (bool, string)
}

// RaijinRunner runs the real raijin binary.
type RaijinRunner struct{}

// Run executes the raijin CLI; returns true when raijin exits 0.
func (RaijinRunner) Run(dir, workflow string, report bool) (bool, string) {
	args := []string{"run", workflow}
	if report {
		args = append(args, "--report")
	}
	cmd := exec.Command("raijin", args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	out, err := cmd.CombinedOutput()
	return err == nil, string(out)
}

// Dispatcher runs all workflows via the runner. Returns the list of
// workflows that failed locally (empty = all passed).
type Dispatcher struct {
	Runner Runner
	Report bool
}

// RunAll executes every workflow in dir. Output is streamed per workflow.
// Returns (failed []string, err) — err is non-nil only for setup problems
// (no workflows found, raijin missing), not for failing workflows.
func (d Dispatcher) RunAll(dir string) ([]string, error) {
	workflows, err := FindWorkflows(dir)
	if err != nil {
		return nil, err
	}
	if len(workflows) == 0 {
		return nil, fmt.Errorf("ci: no workflows found in %s/.github/workflows", dir)
	}
	if _, err := exec.LookPath("raijin"); err != nil {
		return nil, fmt.Errorf("ci: raijin not found in PATH — install it: go install github.com/SaltKing0/raijin@latest")
	}

	var failed []string
	for _, wf := range workflows {
		ok, out := d.Runner.Run(dir, wf, d.Report)
		fmt.Print(out)
		if !ok {
			failed = append(failed, wf)
		}
	}
	return failed, nil
}

// HasWorkflows is a convenience for the hook hint.
func HasWorkflows(dir string) bool {
	files, err := FindWorkflows(dir)
	return err == nil && len(files) > 0
}

// FormatFailed renders a human summary of failed workflows.
func FormatFailed(failed []string) string {
	if len(failed) == 0 {
		return ""
	}
	return fmt.Sprintf("ci: %d workflow(s) failed locally:\n  %s", len(failed), strings.Join(failed, "\n  "))
}
