# go-quality-assurance

Opinionated quality-assurance CLI for Go modules: formatting, vet, static
analysis, vulnerabilities, race-enabled tests and coverage behind one command.

A Go port of [`rails-quality-assurance`](https://github.com/develoz-com/rails-quality-assurance)
and [`js-quality-assurance`](https://github.com/develoz-com/js-quality-assurance),
without their packaging and hook machinery.

> Status: pre-1.0. Hooks and advanced reporting are intentionally deferred.

## Checks

| Command | Tool | Notes |
| --- | --- | --- |
| `gqa format` | `gofmt -l` | fails when any file would be rewritten |
| `gqa vet` | `go vet ./...` | |
| `gqa static` | `staticcheck ./...` | pinned version, installed on demand |
| `gqa vuln` | `govulncheck ./...` | pinned version, installed on demand |
| `gqa test` | `go test -race ./...` | race detector needs cgo and a C toolchain |
| `gqa coverage` | `go test -race -covermode=atomic` | statements, threshold from configuration |
| `gqa ci` | all of the above | sequential, fail-fast |

Every check also runs on its own. `gqa ci` stops at the first failing gate.

## Requirements

Go 1.26 or newer, pinned by `.go-version` and the `go` directive in `go.mod`.
`gqa test` and `gqa coverage` pass `-race`, so the host also needs a working C
toolchain.

`gqa static` and `gqa vuln` need network access and a writable user cache
directory the first time they run, because they install the pinned tools there.

## Installation

```sh
go install github.com/develoz-com/go-quality-assurance/cmd/gqa@latest
```

Or run it without installing:

```sh
go run github.com/develoz-com/go-quality-assurance/cmd/gqa@latest ci
```

## Adoption

Run the gates locally before pushing:

```sh
gqa ci
```

Then make CI enforce the same command. The workflow in this repository is the
reference:

```yaml
- uses: actions/setup-go@v7
  with:
    go-version-file: .go-version
- run: go run github.com/develoz-com/go-quality-assurance/cmd/gqa@latest ci
```

Pin the consumer to a released tag in CI. `staticcheck` and `govulncheck` are
pinned by the tool in its own `go.mod`, and `gqa` installs exactly those
versions into its cache (`<user cache dir>/gqa/tools`, keyed by tool version
and platform) on first use. A consumer therefore needs no `tool` directives of
its own, and every consumer runs the same versions regardless of what its own
`go.mod` records.

The default coverage threshold is 100%. A project with older code states its
own threshold in `qa.yaml`; this repository does the same.

## Configuration

Precedence: environment > project configuration > default.

| Setting | Environment | `qa.yaml` | Default |
| --- | --- | --- | --- |
| Coverage minimum (%) | `QA_COVERAGE_MINIMUM` | `coverage.minimum` | `100` |
| Configuration file | `QA_CONFIG` | — | `qa.yaml` in the module root |

```yaml
coverage:
  minimum: 85
```

`QA_CONFIG` accepts an absolute path or a path relative to the module root.
Unknown keys are configuration errors.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | every gate passed |
| `1` | a gate found problems in the code |
| `2` | configuration or usage error |
| `3` | a gate could not run: missing tool, tool crash, unreadable profile |

This is a deliberate difference from the sibling CLIs, which propagate a tool's
own exit status. Distinct codes let CI and hooks tell a real finding from a
broken environment.

## Development

```sh
go run ./cmd/gqa ci
go test -race ./...
```

The repository dogfoods the CLI in CI, so `make`-style wrappers are unnecessary.

## Release

Tag `main` with `vMAJOR.MINOR.PATCH` and push the tag. Consumers install the
tag; there is no registry step.

## Deferred

- Pre-commit hook installation.
- SARIF, JUnit and merged reporting.
- Language-agnostic preset distribution.

## License

MIT
