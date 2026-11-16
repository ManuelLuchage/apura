# Roadmap

## Phase 1: foundations (done)

- [x] fish integration: wrapper, preexec/postexec, optional prompt segment, completions
- [x] `start` / `resume` / `end -r` / `status` / `tag` / `delete`
- [x] Automatic recording with exit code, duration, start cwd, shell
- [x] `note` (last step, `--step N`, `--session`) and `mark ok|fail|skip|key|none` (with ranges)
- [x] `rm` (numbers, ranges, `--failed`, `--dry-run`)
- [x] `ls` / `search` / `show` filtered by tag, technology, state, result and outcome; `--json`
- [x] Technology inference and secret redaction
- [x] Deterministic `distill` → Markdown or script (`--show-dropped`, `--keep-noise`)
- [x] Unit tests + e2e with a real fish

## Phase 2: experience

- [ ] **TUI** (`apura edit [session]`) with bubbletea: browse, mark, annotate, delete and
      **reorder** steps; live preview of the distilled result
- [ ] A `position` column separate from `seq`, to reorder without losing the original numbers
- [ ] `apura replay <session> --step | --dry-run` (confirms each step; blocks dangerous commands)
- [ ] Partial command output in exports? (to evaluate: capturing output requires a PTY)
- [ ] Capture file edits (`vim`/`nano`/`$EDITOR`): store a diff for the runbook
- [ ] `apura undo` (undo the last `mark`/`rm`)
- [ ] Colored output (respecting `NO_COLOR`)

## Phase 3: more shells and smarter distillation

- [ ] zsh, then bash, then nushell (see SHELL_INTEGRATION.md)
- [ ] `undo-pairs` rule: `install X` + `remove X` cancel out
- [ ] `superseded` rule: `apt install foo` followed by `apt install -y foo` keeps only the last
      (similarity by program + subcommand + arguments)
- [ ] Per-shell parsers when the forgiving tokenizer is not enough
- [ ] **Optional** LLM layer to suggest ordering and explanations (never required, never sends
      data without consent)

## Phase 4: sharing

- [ ] Export/import sessions (JSON) and publish runbooks
- [ ] Sync between machines (to evaluate: end-to-end encryption, Atuin-style)
- [ ] Releases with goreleaser (Linux/macOS, amd64/arm64), Homebrew tap, AUR
- [ ] Localized CLI messages

## Known limitations

- Steps are numbered in the order they reach the binary. Two instant commands in a row could,
  in theory, arrive swapped (the hook runs in the background). Future mitigation: send a
  timestamp from preexec and order by it.
- Full-screen interactive programs (vim, htop) are recorded as a command, without content.
