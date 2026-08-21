package push

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/SaltKing0/fujin/internal/config"
	"github.com/SaltKing0/fujin/internal/store"
)

// Decider decides whether a remote is healthy. Implemented by the health
// engine and mocked in tests.
type Decider interface {
	// Healthy reports whether the remote with the given name/url is
	// reachable and GitHub (if relevant) is operational.
	Healthy(remote config.Remote) bool
}

// Runner executes git commands. *execRunner is the production implementation.
type Runner interface {
	// GitPush runs `git push <url> <refspec...>` in dir and returns combined output.
	GitPush(dir, url string, refspecs []string) (string, error)
}

type execRunner struct{}

func (execRunner) GitPush(dir, url string, refspecs []string) (string, error) {
	args := append([]string{"push", url}, refspecs...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	// tell the pre-push hook this push is fujin's own — don't intercept
	cmd.Env = append(os.Environ(), "FUJIN_INTERNAL=1")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Result describes one push attempt.
type Result struct {
	Remote   config.Remote
	Failover bool
	Queued   bool // true if the push was queued because all remotes were down
	Output   string
	Err      error
}

// Pusher routes a push to the healthiest remote and maintains the offline
// queue for pushes that cannot be delivered while everything is down.
type Pusher struct {
	Cfg     *config.Config
	Store   *store.Store
	Decider Decider
	Runner  Runner
	Now     func() time.Time
	// Dir is the repo directory for the current push. Detected from Getwd
	// in New. When flushing queued pushes, each push runs in the repo where
	// it was originally enqueued (stored as RepoPath).
	Dir string
}

// New creates a Pusher with production defaults.
func New(cfg *config.Config, st *store.Store, decider Decider) *Pusher {
	dir, _ := os.Getwd()
	return &Pusher{
		Cfg:     cfg,
		Store:   st,
		Decider: decider,
		Runner:  execRunner{},
		Now:     time.Now,
		Dir:     dir,
	}
}

// Push pushes refspecs to the primary if healthy, else the first healthy
// failover, else queues them for later. Returns the result and records it.
func (p *Pusher) Push(refspecs []string) Result {
	// primary first
	if p.Decider.Healthy(p.Cfg.Primary) {
		return p.doPush(p.Dir, p.Cfg.Primary, refspecs, false)
	}

	// try failovers in order
	for _, failover := range p.Cfg.Failover {
		if p.Decider.Healthy(failover) {
			return p.doPush(p.Dir, failover, refspecs, true)
		}
	}

	// everything down — queue the push for later (with repo context)
	res := Result{
		Remote: p.Cfg.Primary,
		Queued: true,
		Err:    fmt.Errorf("all remotes unhealthy — push queued (%d refspecs pending)", len(refspecs)),
	}
	if p.Store != nil {
		for _, ref := range refspecs {
			_ = p.Store.EnqueuePush(ref, p.Dir, p.Cfg.Primary.URL)
		}
	}
	p.record(res, refspecs)
	return res
}

// FlushPending replays all queued pushes, oldest first, while any remote is
// healthy. Failed attempts stay in the queue. Returns the number of pushes
// delivered.
func (p *Pusher) FlushPending() (int, error) {
	if p.Store == nil {
		return 0, nil
	}
	pending, err := p.Store.PendingPushes()
	if err != nil {
		return 0, fmt.Errorf("flush: %w", err)
	}
	if len(pending) == 0 {
		return 0, nil
	}

	// pick a healthy target: primary, else first healthy failover
	var target *config.Remote
	if p.Decider.Healthy(p.Cfg.Primary) {
		t := p.Cfg.Primary
		target = &t
	} else {
		for i := range p.Cfg.Failover {
			if p.Decider.Healthy(p.Cfg.Failover[i]) {
				target = &p.Cfg.Failover[i]
				break
			}
		}
	}
	if target == nil {
		return 0, fmt.Errorf("flush: no healthy remote — %d push(es) still queued", len(pending))
	}

	delivered := 0
	for _, psh := range pending {
		// push in the repo where this refspec was originally enqueued
		// ("" = current dir, pre-migration queue entries)
		dir := psh.RepoPath
		if dir == "" {
			dir = p.Dir
		}
		_, err := p.Runner.GitPush(dir, target.URL, []string{psh.Refspec})
		status := "ok"
		if err != nil {
			status = "failed"
		} else {
			delivered++
		}
		_ = p.Store.MarkPushResult(psh.ID, status)
	}
	return delivered, nil
}

func (p *Pusher) doPush(dir string, remote config.Remote, refspecs []string, failover bool) Result {
	out, err := p.Runner.GitPush(dir, remote.URL, refspecs)
	res := Result{
		Remote:   remote,
		Failover: failover,
		Output:   out,
		Err:      err,
	}
	p.record(res, refspecs)
	return res
}

func (p *Pusher) record(res Result, refspecs []string) {
	if p.Store == nil {
		return
	}
	status := "ok"
	errMsg := ""
	if res.Err != nil {
		status = "failed"
		errMsg = res.Err.Error()
	}
	_ = p.Store.SavePush(store.PushRecord{
		Remote:   res.Remote.Name,
		Refspec:  strings.Join(refspecs, " "),
		Status:   status,
		ErrMsg:   errMsg,
		PushedAt: p.Now(),
		Failover: res.Failover,
	})
}
