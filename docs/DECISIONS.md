# Decisions (lightweight ADRs)

### D1: Go
Fastest path to a usable MVP; single binary; easy cross-compilation; bubbletea for the TUI.
Performance is not a factor: the per-command work is one SQLite insert.

### D2: SQLite with `modernc.org/sqlite` by default
Pure Go, no CGO, trivial cross-compilation. `-tags sqlite_cgo` switches to `mattn/go-sqlite3`
(one file per driver in `internal/store/driver_*.go`). Connection: WAL, `busy_timeout=5000`,
`foreign_keys=ON`, one connection per process.

### D3: our own tokenizer instead of `mvdan.cc/sh`
`mvdan.cc/sh` parses POSIX/bash/mksh only and would fail on fish syntax (`; and`, `(cmd)`,
`set -x`). Since the first shell is fish and one database will hold several shells, we use a
forgiving tokenizer that never fails. Per-shell parsers may come later (see SHELL_INTEGRATION.md).

### D4: wrapper + `APURA_EMIT_ENV` to change the parent shell
The direnv/zoxide pattern. Each shell only needs to know how to set and unset variables
(`SetEnv`/`UnsetEnv`); the logic lives in the binary.

### D5: stable `seq` with gaps; hard delete
References the user has seen (`#7`) never change. A future reorder will use a separate `position`
column. `rm` deletes for good (it has `--dry-run`); deleting a session requires `--yes`.

### D6: the manual mark wins over the exit code
The exit code is a guess (`grep` with no match returns 1; `curl` with an HTTP 500 returns 0).
The user's judgement always decides.

### D7: deterministic distillation, no AI in the core
Same input, same output; works offline; auditable (`--show-dropped`). AI may come later as an
optional layer on top.

### D8: English everywhere
apura is an open source project for an international community: code, CLI messages, generated
runbooks, documentation, issues and PRs are all in English. CLI localization is on the roadmap.

### D9: secrets masked before storing
We prefer false positives (fix the runbook by hand) over false negatives (a credential written to
disk).

### D10: the name "Apura"
"Trail" collided with existing binaries (TrailBase, OpsTrail…), and common English words
(marrow, pith, keel, bone) either already had a CLI with the same command or carried problematic
connotations. *Apurar* (Portuguese) means to refine, to purify, to get to the truth, which
describes the core differentiator (`distill`). When chosen, the name was free on npm, PyPI and
crates.io, with no relevant software of the same name and no known offensive connotations
(in Spanish, "¡apura!" means "hurry up", which is colloquial and harmless).
