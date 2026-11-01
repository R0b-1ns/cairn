# cairn update

Replaces the running `cairn` binary with the latest release, after checking its SHA-256.

## Usage

```
cairn update [--check] [--version TAG]
```

## Arguments

None. A positional argument gives exit code 2.

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--check` | `false` | flag, no value | Only report whether a release other than the installed one exists; install nothing. |
| `--version` | (empty) | a release tag such as `v1.0.1` (the `v` is optional) | Install this release instead of the latest. Any other form gives exit code 2. |

## What it does

1. Finds the latest release tag from `https://github.com/R0b-1ns/cairn/releases/latest` (skipped with `--version`).
2. If that version is the installed one, prints `cairn X is up to date` and stops.
3. Downloads `SHA256SUMS` and `cairn-<os>-<arch>` of that release, hashing the binary while it downloads. Only macOS and Linux on amd64 and arm64 have releases.
4. Compares the hash with the `SHA256SUMS` line for that file. On a mismatch nothing is changed.
5. Makes the file executable and renames it over the running binary. The rename is atomic: the binary is either the old one or the new one.

The temporary file is created in the same folder as the binary, so that folder must be writable by the current user. If it is not (for example `/usr/local/bin` owned by root), the command fails with exit code 1 and nothing changes; rerun it with `sudo` or reinstall with `scripts/install.sh`.

## Output

Standard output is one line:

```
updated cairn 1.0.0 -> 1.2.0
cairn 1.2.0 is up to date
cairn 1.2.0 is available (installed: 1.0.0); run 'cairn update'
```

## Exit codes

- `0`: updated, already up to date, or `--check` reported.
- `1`: runtime error: development build, no network, release or asset not found, checksum mismatch, binary folder not writable.
- `2`: usage error: an argument was given, or `--version` is not a release tag.

## Examples

```
cairn update --check
```

Prints whether a different release exists. Exit code 0.

```
cairn update --version v1.0.1
```

Installs v1.0.1, even if it is older than the installed version.

## Notes

- A build made without a release version (`go install`, `go build`) reports itself as `dev`; `cairn update` refuses it with exit code 1, so a source build is never replaced by mistake.
- The checksum comes from the same GitHub release as the binary: it catches a corrupted or truncated download, not a compromised release.
- It never touches notes or databases.
