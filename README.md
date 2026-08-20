# 🌬️ fujin

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
fujin push [refspec...]   # push with automatic failover (default refspec: HEAD)
fujin status              # health of all remotes
fujin log                 # push history (last 20)
```

```text
$ fujin status
✓ primary  github    git@github.com:user/repo.git
✓ failover gitea     git@gitea.example.com:user/repo.git

# GitHub has a major incident:
$ fujin push main
🌬️ fujin: pushed to FAILOVER remote "gitea" (GitHub is having issues)
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

Environment overrides: `FUJIN_PRIMARY_URL`, `FUJIN_DB_PATH`.

**How failover works:** the primary is considered down when the statuspage
indicator is `major`/`critical` **or** both own HTTP checks fail. Push history
(which remote, when, failover or not) is recorded in SQLite and viewable via
`fujin log`.

## Development

```bash
go vet ./...
go test ./... -count=1
```

## Roadmap

- [x] Health-based push routing · status · push log
- [ ] Offline queue (replay when GitHub recovers)
- [ ] Auto mirror setup (create repo on Gitea via API)
- [ ] pre-push hook installer (`fujin install-hook`)

## License

MIT
