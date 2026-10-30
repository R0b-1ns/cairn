# cairn

`cairn` is a small command-line tool that stores **immutable work notes** in SQLite and returns them in small, precise slices. It is built for people and AI agents working on the same project: ask for one task, one repo, one kind of entry or one open question, and spend context only on the answer.

- One static binary, no runtime, no server.
- Notes and addenda cannot be edited or deleted (SQLite triggers reject it). Later facts are added as addenda.
- Two storage modes: a `.cairn/` folder inside the project (shareable through its repository), or a private database in your data directory.
- `cairn guide` prints a one-page guide for AI agents. Every command has `--help`.

## Install

Download the binary for your platform from the [latest release](https://github.com/R0b-1ns/cairn/releases/latest), check it against `SHA256SUMS`, and put it on your `PATH`:

```sh
os=$(uname -s | tr '[:upper:]' '[:lower:]')   # darwin or linux
arch=$(uname -m); case "$arch" in x86_64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; esac
base=https://github.com/R0b-1ns/cairn/releases/latest/download

curl -fsSLO "$base/cairn-$os-$arch"
curl -fsSLO "$base/SHA256SUMS"
grep " cairn-$os-$arch\$" SHA256SUMS | shasum -a 256 -c - && \
  chmod +x "cairn-$os-$arch" && \
  sudo mv "cairn-$os-$arch" /usr/local/bin/cairn && \
  cairn --version        # on Linux use: sha256sum -c -
```

Supported platforms: macOS and Linux, on amd64 and arm64. With Go 1.27 or newer you can also build from source: `go install github.com/R0b-1ns/cairn/cmd/cairn@latest`.

## Quick start

```sh
cd ~/work/tidepool
cairn init --name tidepool --repos server,client --context "Tide data service."

# Write a note at the end of a piece of work
cairn add --agent builder --repo server --task TP-03 --model claude-opus-5-5 \
  --title "Add tidal index" --status done \
  --change "Added the hourly tidal index table." \
  --decision "user:Index buckets are hourly, to match the sensor rate." \
  --open "Should old buckets be pruned after 90 days?" \
  --none note,helper --impact client

# Later, read only what you need
cairn brief TP-03                       # everything about a task
cairn list --type open                  # unresolved open points
cairn search "tidal index"              # full-text search
cairn show n-4k7q2m                     # one full note

# Add to a note without changing it
cairn amend n-4k7q2m --agent builder --model claude-opus-5-5 \
  --resolves e-5t8w1k --note "Pruning runs in the nightly job."
```

The IDs above are examples; every command prints the real ones.

## Documentation

Reference pages are written for AI models: exact names, tables of defaults and allowed values, copy-pasteable examples.

- [Guide](docs/guide.md) — what `cairn guide` prints
- Commands: [docs/commands/](docs/commands/)
- [Data model](docs/data-model.md), [note JSON](docs/note-json.md), [storage](docs/storage.md), [configuration](docs/configuration.md), [environment](docs/environment.md), [limits](docs/limits.md), [exit codes](docs/exit-codes.md), [output](docs/output.md)
- [Security](docs/security.md), [release process](docs/release.md)

## License

MIT. See [LICENSE](LICENSE).
