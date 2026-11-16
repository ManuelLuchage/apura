# Shell integration

fish is the first supported shell. Every other shell plugs in through the same contract, without
touching `core`, `store`, `distill` or the CLI commands.

## The contract

### 1. Environment variables (defined in `internal/core`)

| Variable | Set by | Meaning |
|---|---|---|
| `APURA_SHELL` | init script | shell name (`fish`, `zsh`…) |
| `APURA_SESSION` | wrapper (via `start`/`resume`) | id of the active session in this shell |
| `APURA_SESSION_NAME` | wrapper | session name (for prompts) |
| `APURA_EMIT_ENV` | wrapper, for that call only | asks the binary to print shell code on stdout |
| `APURA_DB` | user (optional) | alternative database path |

### 2. The `apura` wrapper

For `start`, `resume` and `end`: run `APURA_EMIT_ENV=<shell> <bin> "$@"`, evaluate its stdout in
the current shell and return the binary's exit code. Every other subcommand is passed straight
through.

### 3. The recording hook

After **every interactive command**, if `APURA_SESSION` is set:

```
<bin> _record --session $APURA_SESSION --exit <status> --duration <ms> \
              --cwd <directory WHERE THE COMMAND STARTED> --shell <name> -- <command line>
```

- Capture the `status` **before** running anything else in the hook.
- Remember the cwd in a pre-exec hook (a `cd` changes `$PWD` before post-exec).
- Run in the background, with no output (the prompt never waits).
- `--duration` in milliseconds; if the shell doesn't provide it, measure in pre/post or send `0`.
- The binary already ignores empty lines, lines starting with a space and `apura` commands.

### 4. The Go interface (`internal/shell/shell.go`)

```go
type Integration interface {
    Name() string
    InitScript(bin string) string
    SetEnv(key, value string) string
    UnsetEnv(key string) string
    Quote(s string) string
    ScriptHeader() string
    StepGuard() string
    FenceLang() string
}
```

## Checklist for a new shell

1. `internal/shell/<name>/<name>.go`: a type implementing `Integration` + `init(){ shell.Register(...) }`
2. `internal/shell/<name>/apura.<ext>`: script embedded with `//go:embed`; the `@APURA_BIN@`
   placeholder is replaced by the binary path, already quoted
3. A blank import in `internal/shell/all/all.go`
4. Completions: `writeCompletions` in `internal/cli/shell_cmds.go` (cobra generates bash, zsh,
   fish and powershell)
5. Tests: quoting (including a round trip through the real shell when installed), presence of the
   contract elements in the init script, the script passes the shell's syntax-check mode
6. An e2e script (`scripts/e2e_<name>.py`) based on `e2e_fish.py`
7. Update the README and the `init` command's `Long` help

## Sketches for the next shells

**zsh**: native hooks.

```zsh
export APURA_SHELL=zsh
zmodload zsh/datetime
autoload -Uz add-zsh-hook
__apura_preexec() { __apura_cmd=$1; __apura_cwd=$PWD; __apura_t0=$EPOCHREALTIME }
__apura_precmd() {
  local rc=$?
  [[ -n $APURA_SESSION && -n $__apura_cmd ]] || return
  local dur=$(( (EPOCHREALTIME - __apura_t0) * 1000 ))
  ( @APURA_BIN@ _record --session $APURA_SESSION --exit $rc --duration ${dur%.*} \
      --cwd $__apura_cwd --shell zsh -- $__apura_cmd &>/dev/null & )
  unset __apura_cmd
}
add-zsh-hook preexec __apura_preexec
add-zsh-hook precmd  __apura_precmd
apura() { case $1 in start|resume|end) eval "$(APURA_EMIT_ENV=zsh @APURA_BIN@ "$@")";; *) @APURA_BIN@ "$@";; esac }
```
`SetEnv` → `export K=<quoted>`, `UnsetEnv` → `unset K`, `ScriptHeader` → `#!/usr/bin/env zsh` +
`set -euo pipefail`, `StepGuard` → `""`.

Careful: `eval "$(…)"` loses the binary's exit code; keep it with
`local out; out=$(…) || return $?; eval "$out"`.

**bash**: no native preexec. Use `PROMPT_COMMAND` (post) and a `DEBUG` trap with a flag (pre),
or bundle [bash-preexec](https://github.com/rcaloras/bash-preexec). The command line comes from
`history 1` (requires `HISTCONTROL` without `ignoredups` during the session) or from
`$BASH_COMMAND` in the trap. bash 5+ has `$EPOCHREALTIME` for the duration.

**nushell**: `$env.config.hooks.pre_execution` / `pre_prompt`; the exit code is in
`$env.LAST_EXIT_CODE`; the command line in `commandline` (pre-execution).

## Command parsing

`internal/cmdparse` is a forgiving tokenizer that accepts fish and POSIX syntax at the same time,
because steps from different shells coexist in the same database. If a shell ever needs exact
parsing (e.g. `mvdan.cc/sh` for bash/zsh), add a per-shell parser behind the same functions
(`Programs`, `FirstProgram`), selected by the `Step.Shell` field.
