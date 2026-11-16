# apura

> **Refine messy terminal sessions into the steps that worked.**

*Apurar* (Portuguese): to refine, to purify, to get to the truth.

Try things freely in your terminal. `apura` records every command inside a **named session**,
lets you **comment** on and **judge** each step (ok, fail, skip, key), **find** old sessions by
technology, command or note, and **distill** a session full of attempts into the shortest
runbook that actually works.

Shells: **fish** (more on the way, see [`docs/SHELL_INTEGRATION.md`](docs/SHELL_INTEGRATION.md)).

## Why

Shell history remembers *everything*: the typos, the dead ends, the twelfth try. When you need to
do it again (or write it down for a teammate), you dig through it by hand. `apura` keeps the
context (what you were trying to do, what worked, what didn't, and why), then gives you the clean
path.

## Install

```fish
go install github.com/ManuelLuchage/apura/cmd/apura@latest
echo 'status is-interactive; and apura init fish | source' >> ~/.config/fish/config.fish
exec fish
```

Optional: call `apura_prompt` in your `fish_right_prompt` to show the active session.

## Example

```fish
apura start setup-k3s -g "single-node k3s on Hetzner" -t k3s,hetzner
curl -sfL https://get.k3s.io | sh -          # fails: needs sudo
apura note "needs sudo"
curl -sfL https://get.k3s.io | sudo sh -
sudo k3s kubectl get nodes
apura mark key                               # essential step
apura end -r success

apura show setup-k3s                         # every step, in order
apura show setup-k3s --only success
apura distill setup-k3s -o setup-k3s.md      # clean runbook
apura distill setup-k3s -f sh -o setup.fish  # executable script
apura ls --tech kubernetes
apura search "get.k3s.io"
```

Commands starting with a **space** are not recorded (handy for private things). Obvious secrets
(`*_TOKEN=…`, `--password …`, `Authorization: …`, GitHub/AWS tokens…) are masked **before**
they are stored. Everything stays on your machine, in a local SQLite database.

## Commands

| Session | |
|---|---|
| `apura start <name> [-g goal] [-t tags]` | start recording in this shell |
| `apura resume <name>` | record again (also attaches a second terminal) |
| `apura end [-r success\|failure\|abandoned]` | stop and record the result |
| `apura status` · `apura tag <tags…>` · `apura delete <name> --yes` | |

| Steps | |
|---|---|
| `apura note <text> [--step N \| --session]` | comment the last step (or another one, or the session) |
| `apura mark ok\|fail\|skip\|key\|none [N 3-5 …]` | your verdict (wins over the exit code) |
| `apura rm [N …] [--failed] [--dry-run]` | remove steps |

| Find & export | |
|---|---|
| `apura ls [--tag] [--tech] [--active\|--ended] [-r result] [--json]` | |
| `apura search <text> [--tech] [--tag]` | search names, goals, tags, commands and notes |
| `apura show [session] [--only success,fail,key…] [--json]` | |
| `apura distill [session] [-f md\|sh] [-o file] [--show-dropped] [--keep-noise]` | |

Symbols: `★` key · `✔` ok · `✘` fail · `⊘` skip · `·` unmarked, exit 0 · `!` unmarked, exit ≠ 0

## How distill works

Deterministic, offline and auditable (`--show-dropped` explains every removal):

1. drops failed and skipped steps (your marks win over exit codes)
2. drops read-only/navigation noise (`ls`, `cat`, `cd`…) unless marked `key`
3. collapses consecutive repeats of the same command

Directory changes are re-created from each step's working directory.

## Status

Early stage (phase 1). See the [roadmap](docs/ROADMAP.md). Feedback and contributions are very
welcome: see [CONTRIBUTING.md](CONTRIBUTING.md).

## Development

```sh
make setup test e2e
make dev-shell      # fish using the local binary and an isolated DB
```

Database: `$APURA_DB` or `~/.local/share/apura/apura.db`.
Architecture and design decisions live in [`docs/`](docs/).

## License

Licensed under the [Apache License 2.0](LICENSE). See [NOTICE](NOTICE).
