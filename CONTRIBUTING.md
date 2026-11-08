# Contributing

`prompt-backlog` is a single-maintainer project. Issues and pull requests are welcome, but review and fixes are best effort. There is no guaranteed response time or support commitment.

## Before you start

- Open an issue before a large change or a new feature.
- Keep each pull request focused on one concern.
- Report security vulnerabilities through [SECURITY.md](SECURITY.md), not a public issue.
- Do not include credentials, personal data, or real prompt contents in issues, pull requests, tests, or commits.

## Scope

The project is local-only. Contributions that add network calls, vendor automation, credential storage, browser control, or features that bypass or reduce vendor limits are out of scope. See the Scope and Non-Goals section in [README.md](README.md).

## Requirements

- Go 1.26.3 or later
- Git
- Task, optional but recommended

## Setup

```bash
git clone https://github.com/balinomad/prompt-backlog.git
cd prompt-backlog
task setup
```

`task setup` installs `golangci-lint` and `govulncheck`. It uses the latest versions of both tools. CI pins its own versions.

`task setup:release` installs GoReleaser. It is only needed for local release checks.

## Validation

Run the full local gate before pushing:

```bash
task check
```

This runs `go mod tidy -diff`, build, lint, vulnerability scan, and the race-enabled test suite with no cache and shuffled order. It does not cover macOS, Windows, or release packaging; CI covers those.

Useful individual tasks:

- `task test:full`: verbose race tests with no cache and shuffled order.
- `task lint`: run `golangci-lint` with `.golangci.yml`.
- `task vuln`: scan with the local Go toolchain.
- `task vuln:ci`: scan with the Go toolchain from `go.mod`.
- `task cover`: write an HTML coverage report to `.cover/`.

## Code conventions

- Use standard Go formatting. Run `gofmt` on changed files.
- Exported identifiers need GoDoc comments that start with the identifier name and end with a period.
- Tests are table-driven and call `t.Parallel()` at every level where it is safe.
- UI code reads and mutates through `internal/service`, never through `model.Registry` directly.
- JSON tags belong only to DTOs in `internal/store/json`. Model structs stay free of serialization tags.
- Do not add comments unless they explain non-obvious behavior or document a public contract.
- No cascading deletes. Enforce the dependency order described in [README.md](README.md).

## Pull requests

- Explain what changed and why.
- Include tests for behavior changes and bug fixes.
- Update [README.md](README.md), the relevant file in `docs/`, and [CHANGELOG.md](CHANGELOG.md) when usage or behavior changes.
- Keep commit messages imperative and focused.
- The maintainer may edit, squash, or reject a change.

## Releases

Releases are tagged from `master` by the maintainer. The process is documented in [docs/releasing.md](docs/releasing.md). Contributors do not need to create tags or releases.
