package hook

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SaltKing0/fujin/internal/config"
	"github.com/SaltKing0/fujin/internal/push"
)

const hookScript = `#!/bin/sh
# fujin pre-push hook — installed by 'fujin install-hook'
# Routes the push through fujin's failover logic when the target remote is down.
if [ -n "$FUJIN_INTERNAL" ]; then
  exit 0
fi
exec %s hook pre-push "$@"
`

// Install writes the pre-push hook into the current repository
// (.git/hooks/pre-push) using the absolute path of the running fujin binary.
func Install(repoRoot string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("hook: locate fujin binary: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("hook: absolute path: %w", err)
	}

	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return "", fmt.Errorf("hook: create hooks dir: %w", err)
	}
	path := filepath.Join(hooksDir, "pre-push")
	content := fmt.Sprintf(hookScript, abs)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return "", fmt.Errorf("hook: write %s: %w", path, err)
	}
	return path, nil
}

// Uninstall removes the pre-push hook if it was installed by fujin.
func Uninstall(repoRoot string) error {
	path := filepath.Join(repoRoot, ".git", "hooks", "pre-push")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("hook: read %s: %w", path, err)
	}
	if !strings.Contains(string(data), "fujin") {
		return fmt.Errorf("hook: %s exists but was not installed by fujin — refusing to remove", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("hook: remove %s: %w", path, err)
	}
	return nil
}

// Ref is one line from the pre-push hook's stdin:
// "<local ref> <local oid> <remote ref> <remote oid>"
type Ref struct {
	LocalRef  string
	LocalOID  string
	RemoteRef string
	RemoteOID string
}

// ParseRefs reads ref lines from the pre-push hook stdin.
func ParseRefs(reader *bufio.Reader) ([]Ref, error) {
	var refs []Ref
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) == 4 {
				refs = append(refs, Ref{
					LocalRef:  fields[0],
					LocalOID:  fields[1],
					RemoteRef: fields[2],
					RemoteOID: fields[3],
				})
			}
		}
		if err != nil {
			break // io.EOF
		}
	}
	return refs, nil
}

// PrePush handles `fujin hook pre-push <remote-name> <remote-url>`.
// It reads pushed refs from stdin and:
//   - lets the push through when the target remote is healthy,
//   - pushes to the first healthy failover itself and then blocks the
//     original push (exit code 1) when the target is down.
func PrePush(cfg *config.Config, decider push.Decider, runner push.Runner, remoteName, remoteURL string, refs []Ref) (int, string) {
	// find the target remote in the config
	target := findRemote(cfg, remoteName, remoteURL)
	if target == nil {
		// not one of our remotes — let git handle it
		return 0, ""
	}

	if decider.Healthy(*target) {
		return 0, ""
	}

	// target is down — try to push the refs to the first healthy failover
	var localRefs []string
	for _, r := range refs {
		if r.LocalRef != "" {
			localRefs = append(localRefs, r.LocalRef)
		}
	}
	if len(localRefs) == 0 {
		return 1, "fujin: target remote is unhealthy and no local refs to fail over"
	}

	for i := range cfg.Failover {
		fo := cfg.Failover[i]
		if fo.Name == target.Name {
			continue // don't fail over to itself
		}
		if !decider.Healthy(fo) {
			continue
		}
		out, err := runner.GitPush(fo.URL, localRefs)
		if err != nil {
			return 1, fmt.Sprintf("fujin: failover push to %q failed: %v\n%s", fo.Name, err, out)
		}
		msg := fmt.Sprintf("🌬️ fujin: %s is down — pushed to FAILOVER remote %q instead\n%s",
			target.Name, fo.Name, out)
		return 1, msg + failoverHint()
	}

	return 1, fmt.Sprintf("fujin: target remote %q is unhealthy and no failover remote is reachable", target.Name)
}

// failoverHint suggests running CI locally via raijin when GitHub Actions is
// affected. Returns "" when raijin is not installed or no workflows exist.
func failoverHint() string {
	if _, err := exec.LookPath("raijin"); err != nil {
		return ""
	}
	files, err := filepath.Glob(".github/workflows/*.yml")
	if err != nil || len(files) == 0 {
		files, err = filepath.Glob(".github/workflows/*.yaml")
		if err != nil || len(files) == 0 {
			return ""
		}
	}
	return fmt.Sprintf("🌩️ Hinweis: CI läuft nicht auf GitHub — lokalen Run starten:\n   fujin ci\n")
}

func findRemote(cfg *config.Config, name, url string) *config.Remote {
	for i := range cfg.AllRemotes() {
		r := &cfg.AllRemotes()[i]
		if name != "" && r.Name == name {
			return r
		}
		if url != "" && r.URL == url {
			return r
		}
	}
	return nil
}

// GitPushWithEnv runs `git push <url> <refs...>` with FUJIN_INTERNAL=1 so the
// pre-push hook does not intercept fujin's own pushes.
func GitPushWithEnv(url string, refs []string) (string, error) {
	args := append([]string{"push", url}, refs...)
	cmd := exec.Command("git", args...)
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), "FUJIN_INTERNAL=1")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// RunnerFunc adapts a plain function to the push.Runner interface.
type RunnerFunc func(url string, refs []string) (string, error)

func (f RunnerFunc) GitPush(url string, refs []string) (string, error) {
	return f(url, refs)
}
