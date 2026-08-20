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
	// GitPush runs `git push <url> <refspec...>` and returns combined output.
	GitPush(url string, refspecs []string) (string, error)
}

type execRunner struct{}

func (execRunner) GitPush(url string, refspecs []string) (string, error) {
	args := append([]string{"push", url}, refspecs...)
	cmd := exec.Command("git", args...)
	cmd.Stdin = os.Stdin
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Result describes one push attempt.
type Result struct {
	Remote   config.Remote
	Failover bool
	Output   string
	Err      error
}

// P usher routes a push to the healthiest remote.
type Pusher struct {
	Cfg     *config.Config
	Store   *store.Store
	Decider Decider
	Runner  Runner
	Now     func() time.Time
}

// New creates a Pusher with production defaults.
func New(cfg *config.Config, st *store.Store, decider Decider) *Pusher {
	return &Pusher{
		Cfg:     cfg,
		Store:   st,
		Decider: decider,
		Runner:  execRunner{},
		Now:     time.Now,
	}
}

// Push pushes refspecs to the primary if healthy, else the first healthy
// failover. Returns the result and records it in the store.
func (p *Pusher) Push(refspecs []string) Result {
	// primary first
	if p.Decider.Healthy(p.Cfg.Primary) {
		return p.doPush(p.Cfg.Primary, refspecs, false)
	}

	// try failovers in order
	for _, failover := range p.Cfg.Failover {
		if p.Decider.Healthy(failover) {
			return p.doPush(failover, refspecs, true)
		}
	}

	// everything down
	res := Result{
		Remote: p.Cfg.Primary,
		Err:    fmt.Errorf("all remotes unhealthy — GitHub and %d failover(s) unreachable", len(p.Cfg.Failover)),
	}
	p.record(res, refspecs)
	return res
}

func (p *Pusher) doPush(remote config.Remote, refspecs []string, failover bool) Result {
	out, err := p.Runner.GitPush(remote.URL, refspecs)
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
