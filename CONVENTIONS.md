# Conventions

## Repository

- Default branch: `main`.
- Public repository under the `develoz-com` organisation.
- Work is tracked in Linear, team Race Control; commit and PR titles reference
  the issue identifier.
- Durable documentation lives in `README.md` and `docs/`; no wiki.

## Engineering

- Go 1.26, pinned by `.go-version` and the `go` directive in `go.mod`.
- Tool versions are pinned with `tool` directives in `go.mod`. Never pin with
  install scripts, `latest`, or unversioned branches. `gqa` installs the pinned
  versions into its own cache, so consumers need no directives of their own.
- Standard library first; add a dependency only when it removes more code than
  it adds.
- Tests are table-driven and use only the standard library.
- Every file ends with a trailing newline.

## Quality gates

- The single entrypoint is `go run ./cmd/gqa ci`, which runs formatting, vet,
  staticcheck, govulncheck, race-enabled tests and coverage.
- CI runs the same command on push and pull request to `main`.
- Fix findings at the source. Do not add tool exclusions to silence them.
- Coverage thresholds are explicit in `qa.yaml`; the tool default of 100% is
  not lowered silently.

## Collaboration

- Branches are created per issue and merged through a pull request against
  `main`.
- Commits follow Conventional Commits (`feat:`, `fix:`, `test:`, `docs:`,
  `ci:`, `release:`).
- A change is reviewed and the gates rerun before merge.
