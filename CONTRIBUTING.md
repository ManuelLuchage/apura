# Contributing to apura

Thanks for your interest! apura is young, so ideas, bug reports and pull requests all help.

## Ways to help

- **Try it and report**: open an issue with your OS, shell version (`fish --version`) and what
  happened. For recording problems, `apura show --json` output is very useful (check it for
  anything private first).
- **Add a shell**: zsh, bash and nushell are wanted. The whole contract and a checklist are in
  [docs/SHELL_INTEGRATION.md](docs/SHELL_INTEGRATION.md); a new shell is one package plus one import.
- **Improve distillation**: new rules implement `distill.Rule` (see
  [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)).
- **Teach it more tools**: extend the program → technology map in `internal/tech`, or the secret
  patterns in `internal/redact` (always with a test case).

## Development setup

Requirements: Go (see `go.mod`), `fish` and `python3` for the end-to-end test.

```sh
git clone https://github.com/ManuelLuchage/apura
cd apura
make setup      # go mod tidy
make test       # unit tests
make e2e        # drives a real interactive fish through a PTY
make dev-shell  # fish using your local build and an isolated database in .dev/
```

## Pull requests

- Keep changes focused; open an issue first for anything large.
- Add tests for new behavior. Touching `internal/shell/fish/apura.fish`? Run `make e2e`.
- Run `make fmt lint test` before pushing.
- Follow the project rules in [CLAUDE.md](CLAUDE.md) (they apply to humans too): shell-specific
  syntax only under `internal/shell/<name>/`, SQL only in `internal/store`, schema changes only via
  new migrations, secrets always redacted, `_record` kept fast and silent.
- Write commit messages, code, comments and docs in English.

## Design decisions

Before proposing changes to fundamentals (storage, the shell contract, distillation being
deterministic), please read [docs/DECISIONS.md](docs/DECISIONS.md).

## License

apura is licensed under the [Apache License 2.0](LICENSE). By submitting a contribution, you agree
that it is licensed under the same terms (Apache-2.0, section 5).
