// Package setup implements the interactive first-run wizard (`fujin init`).
package setup

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/SaltKing0/fujin/internal/config"
)

// prompt reads one line of input, trimming whitespace. Returns the trimmed
// input; empty string on EOF.
func prompt(r *bufio.Reader) string {
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return ""
	}
	return strings.TrimSpace(line)
}

// ask prints a question with a default and returns the answer (default when
// the user just hits enter).
func ask(r *bufio.Reader, w io.Writer, question, def string) string {
	if def != "" {
		fmt.Fprintf(w, "%s [%s]: ", question, def)
	} else {
		fmt.Fprintf(w, "%s: ", question)
	}
	ans := prompt(r)
	if ans == "" {
		return def
	}
	return ans
}

// OriginURL returns the URL of the `origin` remote in dir, or "" if there is
// no origin (or the repo is not a git repo).
func OriginURL(dir string) string {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Init runs the interactive setup wizard. It detects the origin remote and
// asks for a failover remote, then writes the config file.
// Returns the path the config was written to.
func Init(r io.Reader, w io.Writer, configPath string) (string, error) {
	br := bufio.NewReader(r)

	fmt.Fprintln(w, "🌬️ fujin init — set up push failover")
	fmt.Fprintln(w, "   (ctrl+c to abort, empty answers keep the default)")
	fmt.Fprintln(w)

	cfg := config.Default()

	// primary: detect origin if present
	origin := OriginURL(".")
	primaryURL := ask(br, w, "Primary remote URL (your main forge, usually GitHub)", origin)
	if primaryURL == "" {
		return "", fmt.Errorf("primary remote URL is required")
	}
	cfg.Primary.Name = ask(br, w, "Primary remote name", "github")
	cfg.Primary.URL = primaryURL

	// failover: optional, can add more later by editing the config
	failoverURL := ask(br, w, "Failover remote URL (Gitea/Forgejo/GitLab; empty to skip)", "")
	if failoverURL != "" {
		fo := config.Remote{
			Name: ask(br, w, "Failover remote name", "gitea"),
			URL:  failoverURL,
		}
		cfg.Failover = append(cfg.Failover, fo)
	}

	// health endpoints: offer defaults
	ep := ask(br, w, "Health check endpoints (comma-separated)", strings.Join(cfg.Health.Endpoints, ", "))
	if ep != "" {
		var eps []string
		for _, e := range strings.Split(ep, ",") {
			if e = strings.TrimSpace(e); e != "" {
				eps = append(eps, e)
			}
		}
		if len(eps) > 0 {
			cfg.Health.Endpoints = eps
		}
	}

	if err := config.Save(configPath, &cfg); err != nil {
		return "", err
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "✅ wrote %s\n", configPath)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintln(w, "  🌬️ fujin status        # check remote health")
	fmt.Fprintln(w, "  🌬️ fujin push main      # push with failover")
	fmt.Fprintln(w, "  🌬️ fujin install-hook   # route every 'git push' through fujin")
	return configPath, nil
}
