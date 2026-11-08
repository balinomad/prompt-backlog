# Release Publishing

This document describes the release process for maintainers. The release workflow is automated; the maintainer's job is to verify the branch and create the version tag.

## Versioning

Use semantic versioning: `vMAJOR.MINOR.PATCH`.

- Before `v1.0.0`, the minor version may include breaking changes and the project is not yet stable.
- From `v1.0.0`, a breaking change requires a major version, a feature requires a minor version, and a fix requires a patch version.
- Never move or reuse a published tag. Publish a new patch release instead.

## Preconditions

All of the following must be true before tagging:

- The working tree is clean except for files you intentionally leave uncommitted.
- `master` is current with `origin/master`.
- The full local validation passes.
- CI on `master` is green on Linux, macOS, and Windows.
- The README installation instructions still work.
- No known data-loss or persistence bug remains open.

## 1. Prepare the branch

```bash
git switch master
git pull --ff-only
git status --short
```

Expected: no unexpected changes. `data/` is ignored and may exist locally.

## 2. Validate locally

```bash
task check
```

This runs build, lint, vulnerability scan, and the race-enabled test suite. If `task check` is unavailable, run the equivalent commands:

```bash
go build ./...
golangci-lint run ./...
govulncheck ./...
go test -race -count=1 -shuffle=on ./...
```

Then confirm CI is green on `master`:

```text
https://github.com/balinomad/prompt-backlog/actions
```

Do not tag while a required job is failing.

## 3. Dry-run the release build

Install GoReleaser locally if it is missing. Then validate the configuration and build the archives without publishing:

```bash
goreleaser check
goreleaser release --snapshot --clean --skip=publish
```

Inspect `dist/`:

- Five archives exist: Linux amd64 and arm64, macOS amd64 and arm64, Windows amd64.
- Each archive contains the `prompt-backlog` binary, `README.md`, and `LICENSE`.
- `checksums.txt` exists.

`dist/` is ignored by git and must not be committed.

## 4. Create and push the tag

Set `VERSION` to the release version. It must not already exist.

```bash
VERSION=vX.Y.Z
git tag -a "$VERSION" -m "$VERSION"
git push origin "$VERSION"
```

The tag push triggers `.github/workflows/release.yml`. Pushing a branch does not trigger a release.

GoReleaser strips the leading `v` in artifact names, so tag `vX.Y.Z` produces archives named `prompt-backlog_X.Y.Z_...`.

## 5. Monitor the workflow

Open the release run:

```text
https://github.com/balinomad/prompt-backlog/actions/workflows/release.yml
```

The job runs `go test ./...`, `go vet ./...`, builds all targets, writes checksums, generates the changelog, and publishes the GitHub release.

A passing run ends with a release at:

```text
https://github.com/balinomad/prompt-backlog/releases
```

## 6. Verify the published artifacts

On the release page, confirm:

- The release is not a draft.
- Five archives are attached.
- `checksums.txt` is attached.
- The changelog lists the expected commits.
- The source archives are present.

Set `VERSION` to the published tag, then download one archive and verify its checksum.

Linux example:

```bash
VERSION=vX.Y.Z
ARCHIVE="prompt-backlog_${VERSION#v}_linux_amd64.tar.gz"
sha256sum --check checksums.txt --ignore-missing
tar -xzf "$ARCHIVE"
./prompt-backlog -h
```

macOS example, comparing one archive against its checksum line:

```bash
VERSION=vX.Y.Z
ARCHIVE="prompt-backlog_${VERSION#v}_darwin_arm64.tar.gz"
shasum -a 256 "$ARCHIVE"
grep "$ARCHIVE" checksums.txt
```

Windows archives are `.zip`.

## 7. After publishing

- Announce the release only after step 6 passes.
- If the README changed, confirm the published release points to the new instructions.
- Record any release-specific caveat in the release notes.

## Failure handling

If the workflow fails before publishing:

1. Fix the cause on `master`.
2. Delete the local and remote tag.
3. Recreate the tag on the fixed commit.

```bash
VERSION=vX.Y.Z
git tag -d "$VERSION"
git push origin ":refs/tags/$VERSION"
```

The remote tag deletion is destructive. Confirm that no release was published before doing it.

If a release was published with broken artifacts, do not move the tag. Publish a new patch release with the fix. Delete the broken release only after the replacement exists.

## Security notes

- The workflow uses the repository `GITHUB_TOKEN` with `contents: write`.
- No long-lived release token is stored.
- Archives contain no credentials or local data.
- Binaries are unsigned. macOS may quarantine them and Windows SmartScreen may warn; code signing and notarization are separate work.
