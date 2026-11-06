# Plan

## Milestone 1 — reusable Go QA CLI

Status: complete.

Deliverables:

- One `ci` command that runs formatting, vet, static analysis, vulnerabilities,
  race-enabled tests and coverage enforcement, with each gate also callable
  individually.
- Configuration precedence environment > project file > default, default
  coverage 100%, documented and covered by tests.
- Tool versions pinned through `go.mod` tool directives and the Go toolchain
  pinned by `.go-version`.
- Exit codes that separate findings (`1`) from configuration (`2`) and tool
  failures (`3`).
- Installation, release and adoption documented; the CLI dogfoods itself in CI.

## Decisions

- **Tool pins without consumer setup.** `go tool staticcheck` resolves tools
  against the module in the working directory, so a consumer with no `tool`
  directives cannot run it. `gqa` installs the versions its own `go.mod`
  records into a version- and platform-keyed cache (`go install
  <package>@<version>`) and then executes the binaries directly. Consumers need
  no directives, machines do not drift, and the exit statuses the gates classify
  stay intact — unlike `go run <package>@<version>`, which collapses any non-zero
  exit to `1`.
- **Exit-code contract.** The sibling CLIs pass a tool's exit status through.
  Go check tools already spend those codes on their own meanings
  (`govulncheck` returns 3 for findings), so a small fixed contract is clearer
  for callers.
- **YAML config.** One key today (`coverage.minimum`). The file exists so
  consumer thresholds are explicit rather than hidden in CLI flags.
- **No wrappers in the repository.** CI runs `gqa ci` directly; a Makefile
  would only restate it.
- **`ci` is fail-fast.** Matches the sibling pipelines and keeps the output
  short. Gates remain individually runnable for full sweeps.

## Deferred

- Pre-commit hook installation.
- SARIF/JUnit reporting and report merging.
- Optional gates that need extra tooling (dead code, duplication).
- Windows-specific path handling beyond `filepath` defaults.
