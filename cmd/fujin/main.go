package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/SaltKing0/fujin/internal/config"
	"github.com/SaltKing0/fujin/internal/health"
	"github.com/SaltKing0/fujin/internal/hook"
	"github.com/SaltKing0/fujin/internal/push"
	"github.com/SaltKing0/fujin/internal/statuspage"
	"github.com/SaltKing0/fujin/internal/store"
)

// overridden at build time via -ldflags "-X main.version=..."
var version = "0.1.0"

// healthDecider combines the statuspage indicator with own HTTP checks.
type healthDecider struct {
	client  *statuspage.Client
	checker *health.Checker
}

// Healthy reports whether the given remote is usable. The primary (github)
// additionally requires the statuspage indicator to be "none" or "minor".
func (h *healthDecider) Healthy(remote config.Remote) bool {
	// own HTTP checks always run
	results := h.checker.CheckAll()
	allOK := true
	for _, r := range results {
		if r.Err != nil || r.StatusCode >= 500 {
			allOK = false
		}
	}

	if remote.Name == "github" || remote.URL == h.checkerEndpointsPrimary() {
		if st, err := h.client.GetStatus(); err == nil {
			// critical/major => GitHub is having a real incident
			if st.Indicator == statuspage.StatusCritical || st.Indicator == statuspage.StatusMajor {
				return false
			}
		}
	}
	return allOK
}

func (h *healthDecider) checkerEndpointsPrimary() string {
	return ""
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `fujin %s — wind god of Git pushes

Usage:
  fujin push [refspec...]   push with automatic failover (flushes queue first)
  fujin flush               replay queued pushes (when a remote is healthy again)
  fujin status              show health of all remotes
  fujin log                 show push history
  fujin --version           print version

Flags:
`, version)
		flag.PrintDefaults()
	}

	var (
		configPath = flag.String("config", "", "path to config file (default: ~/.config/fujin/config.yaml)")
		showVer    = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("fujin %s\n", version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	client := statuspage.NewClient("")
	checker := health.New(cfg.Health.Endpoints, 60*time.Second, st)
	decider := &healthDecider{client: client, checker: checker}

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	switch args[0] {
	case "push":
		refspecs := args[1:]
		if len(refspecs) == 0 {
			refspecs = []string{"HEAD"}
		}
		p := push.New(cfg, st, decider)

		// flush any queued pushes first, while we're here
		if delivered, err := p.FlushPending(); err != nil {
			fmt.Fprintf(os.Stderr, "fujin: flush: %v\n", err)
		} else if delivered > 0 {
			fmt.Printf("🌬️ fujin: flushed %d queued push(es)\n", delivered)
		}

		res := p.Push(refspecs)
		if res.Output != "" {
			fmt.Print(res.Output)
		}
		if res.Queued {
			fmt.Printf("🌬️ fujin: all remotes unhealthy — push QUEUED, will retry on next push/flush\n")
			os.Exit(1)
		}
		if res.Failover {
			fmt.Printf("🌬️ fujin: pushed to FAILOVER remote %q (GitHub is having issues)\n", res.Remote.Name)
		} else {
			fmt.Printf("🌬️ fujin: pushed to %q\n", res.Remote.Name)
		}
		if res.Err != nil {
			fmt.Fprintf(os.Stderr, "fujin: push failed: %v\n", res.Err)
			os.Exit(1)
		}

	case "flush":
		p := push.New(cfg, st, decider)
		delivered, err := p.FlushPending()
		if err != nil {
			fmt.Fprintf(os.Stderr, "fujin: %v\n", err)
			os.Exit(1)
		}
		if delivered == 0 {
			if pending, _ := st.PendingPushes(); len(pending) > 0 {
				fmt.Printf("🌬️ fujin: no healthy remote — %d push(es) still queued\n", len(pending))
				os.Exit(1)
			}
			fmt.Println("🌬️ fujin: queue is empty")
		} else {
			fmt.Printf("🌬️ fujin: flushed %d queued push(es)\n", delivered)
		}

	case "install-hook":
		path, err := hook.Install(".")
		if err != nil {
			fmt.Fprintf(os.Stderr, "fujin: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("🌬️ fujin: pre-push hook installed at %s\n", path)
		fmt.Println("   every 'git push' now routes through fujin's failover logic")

	case "uninstall-hook":
		if err := hook.Uninstall("."); err != nil {
			fmt.Fprintf(os.Stderr, "fujin: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("🌬️ fujin: pre-push hook removed")

	case "hook":
		if len(args) < 2 || args[1] != "pre-push" {
			fmt.Fprintln(os.Stderr, "fujin: usage: fujin hook pre-push <remote-name> <remote-url>")
			os.Exit(2)
		}
		remoteName := ""
		remoteURL := ""
		if len(args) >= 3 {
			remoteName = args[2]
		}
		if len(args) >= 4 {
			remoteURL = args[3]
		}
		refs, err := hook.ParseRefs(bufio.NewReader(os.Stdin))
		if err != nil {
			fmt.Fprintf(os.Stderr, "fujin: parse refs: %v\n", err)
			os.Exit(1)
		}
		code, msg := hook.PrePush(cfg, decider, hook.RunnerFunc(hook.GitPushWithEnv), remoteName, remoteURL, refs)
		if msg != "" {
			fmt.Fprintln(os.Stdout, msg)
		}
		os.Exit(code)

	case "status":
		for _, r := range cfg.AllRemotes() {
			ok := decider.Healthy(r)
			mark := "✓"
			if !ok {
				mark = "✗"
			}
			label := "primary "
			if r.Name != cfg.Primary.Name {
				label = "failover"
			}
			fmt.Printf("%s %s %-8s %s\n", mark, label, r.Name, r.URL)
		}

	case "log":
		records, err := st.RecentPushes(20)
		if err != nil {
			fmt.Fprintf(os.Stderr, "log: %v\n", err)
			os.Exit(1)
		}
		if len(records) == 0 {
			fmt.Println("no pushes recorded yet")
			return
		}
		fmt.Printf("%-20s %-10s %-9s %s\n", "TIME", "REMOTE", "STATUS", "REFSPEC")
		for _, r := range records {
			fo := ""
			if r.Failover {
				fo = " (failover)"
			}
			fmt.Printf("%-20s %-10s %-9s %s%s\n",
				r.PushedAt.Format("2006-01-02 15:04"), r.Remote+fo, r.Status, r.Refspec, "")
		}

	default:
		fmt.Fprintf(os.Stderr, "fujin: unknown command %q\n", args[0])
		flag.Usage()
		os.Exit(2)
	}
}
