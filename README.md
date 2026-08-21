# 🌬️ fujin

[![Go](https://github.com/SaltKing0/fujin/actions/workflows/go.yml/badge.svg)](https://github.com/SaltKing0/fujin/actions/workflows/go.yml)

**Wind god of Git pushes** — a thin CLI that wraps `git push` with automatic
multi-remote failover. When GitHub is down, `fujin push` pushes to your
failover forge (Gitea / Forgejo / GitLab) instead of failing.

Named after Fūjin, the Shinto wind god. When Kagutsuchi (fire) reports the
outage, Fūjin (wind) carries your commits elsewhere.

## Install

```bash
go install github.com/SaltKing0/fujin@latest
```

## Usage

```bash
fujin push [refspec...]   # push with automatic failover (flushes queue first)
fujin flush               # replay queued pushes (when a remote is healthy again)
fujin status              # health of all remotes (add --json for scripting)
fujin queue               # show queued pushes
fujin log                 # push history (last 20)
fujin init                # interactive first-run setup (detects git origin)
fujin install-hook        # install a pre-push hook: every 'git push' uses failover
fujin uninstall-hook      # remove the hook again
```

```text
$ fujin status
✓ primary  github    git@github.com:user/repo.git
✓ failover gitea     git@gitea.example.com:user/repo.git

# GitHub has a major incident:
$ fujin push main
🌬️ fujin: pushed to FAILOVER remote "gitea" (GitHub is having issues)

# GitHub AND your failover are both down:
$ fujin push main
🌬️ fujin: all remotes unhealthy — push QUEUED, will retry on next push/flush

# …later, when a remote is healthy again:
$ fujin flush
🌬️ fujin: flushed 2 queued push(es)
```

## Configuration

`~/.config/fujin/config.yaml`:

```yaml
primary:
  name: github
  url: git@github.com:user/repo.git
failover:
  - name: gitea
    url: git@gitea.example.com:user/repo.git
health:
  statuspage: true                  # consult githubstatus.com
  endpoints:                        # own HTTP checks
    - https://github.com
    - https://api.github.com
```

Environment overrides: `FUJIN_PRIMARY_URL`, `FUJIN_DB_PATH`,
`FUJIN_TELEGRAM_BOT_TOKEN`, `FUJIN_TELEGRAM_CHAT_ID`.

**Telegram alerts:** set `telegram_bot_token` + `telegram_chat_id` and fujin
notifies you when a push gets queued ("Push queued — GitHub is down") and
when queued pushes are flushed. Same bot credentials as kagutsuchi — one bot
for the whole family.

**How failover works:** the primary is considered down when the statuspage
indicator is `major`/`critical` **or** both own HTTP checks fail. Push history
(which remote, when, failover or not) is recorded in SQLite and viewable via
`fujin log`.

**Offline queue:** if every remote is unhealthy, `fujin push` queues the
refspecs in SQLite instead of failing. `fujin flush` (or the next
`fujin push`) replays the queue oldest-first as soon as any remote is healthy
again. Failed replay attempts stay queued for the next retry.

**pre-push hook:** `fujin install-hook` writes the hook into the repo's hooks
dir (worktrees and `core.hooksPath` are honored) so every plain `git push`
routes through the failover logic — when the target remote is down, fujin
pushes your refs to the first healthy failover itself and blocks the original
push with a clear message. Ref mappings (`local:remote`) and deletes are
preserved. fujin's own pushes set `FUJIN_INTERNAL=1` so the hook never
intercepts them.

### Exit codes

| Command | Code | Meaning |
|---|---|---|
| `fujin push` | 0 | pushed (primary or failover) |
| `fujin push` | 1 | all remotes down — push queued |
| `fujin push` | 1 | push failed (remote rejected) |
| `fujin flush` | 0 | queue empty or flushed |
| `fujin flush` | 1 | no healthy remote — still queued |
| `fujin` (all) | 2 | usage error / unknown command |

## Development

```bash
go vet ./...
go test ./... -count=1
```

## Roadmap

- [x] Health-based push routing · status · push log
- [x] Offline queue (replay when a remote recovers)
- [x] pre-push hook installer (`fujin install-hook`)
- [ ] Auto mirror setup (create repo on Gitea via API)

## The kami family

| Project | God | Purpose |
|---|---|---|
| [kagutsuchi](https://github.com/SaltKing0/kagutsuchi) | 🔥 Fire | TUI reliability dashboard |
| [fūjin](https://github.com/SaltKing0/fujin) | 🌬️ Wind | This project — multi-remote push failover |
| [raijin](https://github.com/SaltKing0/raijin) | 🌩️ Thunder | Local CI failover via act |
| [ghhealth](https://github.com/SaltKing0/ghhealth) | — | Shared health engine (statuspage + health checks) |

## License

MIT
