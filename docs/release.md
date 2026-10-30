# Release process

Every merge into `main` is built and tested; releases are produced from conventional commits.

## What runs

| Workflow | Trigger | What it does | Permissions |
|---|---|---|---|
| `.github/workflows/ci.yml` | push to `main`, every pull request | `go mod verify`, `go vet`, `go test -race`, then `scripts/build.sh` for all four targets; on a push to `main` the binaries are kept as a build artifact for 14 days | `contents: read` |
| `.github/workflows/release-please.yml` | push to `main` | job `release-please` opens or updates the release pull request (version bump and `CHANGELOG.md` from conventional commits); when that pull request is merged, job `build` (read-only token) runs `go mod verify`, the tests and `scripts/build.sh` and uploads `dist/` as an artifact, and job `publish` (the only job that can modify the repository) checks `SHA256SUMS` and attaches the files to the tagged GitHub release | `release-please`: `contents: write` + `pull-requests: write`; `build`: `contents: read`; `publish`: `contents: write` |
| `.github/dependabot.yml` | weekly | pull requests that update pinned GitHub Actions and Go modules | none |

A change that does not build or pass its tests cannot be merged when the `ci` check is required by branch protection.

## Versions

Commits follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:` bumps the minor version, `fix:` the patch version, `feat!:` or a `BREAKING CHANGE:` footer the major version (while the version is below 1.0.0, breaking changes bump the minor version). Tags look like `v0.2.0`.

## Binaries

| File | Platform |
|---|---|
| `cairn-darwin-amd64`, `cairn-darwin-arm64` | macOS |
| `cairn-linux-amd64`, `cairn-linux-arm64` | Linux |
| `SHA256SUMS` | SHA-256 of each binary, one `hash  file` line each |

Build locally with `scripts/build.sh [version]`; it writes `dist/` and prints the checksums. Builds are static (`CGO_ENABLED=0`), use `-trimpath` and `-ldflags "-s -w -buildid="`, so the same source and Go version produce the same bytes.

## Verify a download

```sh
grep " cairn-linux-amd64\$" SHA256SUMS | sha256sum -c -     # macOS: shasum -a 256 -c -
```

Run the check before executing the binary. `SHA256SUMS` is published next to the binaries on the same release.

## Pipeline security

- Every workflow declares `permissions` at the top, set to nothing or read-only; a job gets write access only where it needs it.
- Every third-party action is pinned to a full commit SHA, with the version in a comment. Dependabot proposes updates as reviewable pull requests.
- Tests and builds never run in a job that can modify the repository; the publish job only downloads the built files and attaches them.
- The only secret used is the automatic `GITHUB_TOKEN`. No personal token, signing key or cloud credential is stored.
- Checkouts do not persist credentials. Untrusted event text (titles, branch names, commit messages) is never interpolated into shell commands.
- `internal/ci/workflows_test.go` fails the build if any of these rules is broken.

## Known limitation

Pull requests opened by `GITHUB_TOKEN` (the release pull request) do not start other workflows, so `ci` does not run on the release pull request by itself. Before merging it, push a no-op commit to its branch or close and reopen it to trigger `ci`.
