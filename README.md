# cairn 🪨

**Leave a stone for whoever comes next.**

A cairn is a small pile of stones left on the trail so the next traveler knows the way. This one is for your project: why was it built this way, what changed, what is still open?

Every new session, human or AI, starts by asking those questions again. `cairn` is a tiny command-line tool where people and agents drop short, permanent notes as they work, then ask for exactly the slice they need (one task, one repo, one open question) instead of rereading everything.

- **One file, zero setup.** A single static binary: no runtime, no server.
- **Can't be rewritten.** Notes are immutable and SQLite itself refuses edits. New facts go in addenda, so the original is never lost.
- **Cheap to read.** `cairn brief TP-03` gives you a whole task in a few lines, always with IDs, so an agent spends its context on the answer.
- **Yours to keep.** Store notes in the project's own `.cairn/` folder and share them through git, or keep them private on your machine.
- **Made for agents.** `cairn guide` prints a one-page guide an AI can read on its own. Every command has `--help`.

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
