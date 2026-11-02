# cairn export

Writes notes as Markdown (to read) or as JSON (to import into another database).

## Usage

```
cairn export [--format md|json] [--task TASK] [--out FILE] [--force] [--project NAME]
```

## Arguments

None. A positional argument gives exit code 2.

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--format` | `md` | `md` (readable), `json` (importable) | Output format. |
| `--task` | (empty) | a task ID matching the project task pattern | Only notes of this task. A base task includes its extensions. |
| `--out` | (empty) | a file path | Write to this file instead of standard output. The file is created with mode 0600. |
| `--force` | `false` | flag, no value | With `--out`: replace an existing regular file. |
| `--project` | (empty) | a registered project name | Project to read. Default: found from the current directory, or env `CAIRN_PROJECT`. |

## Output

Without `--out`, the document goes to standard output. Markdown example (`--format md`, abridged):

```
# tidepool

Tide data service. Sensor ingest, the tidal index and the browser client.

## n-4k7q2m  Add tidal index

task: TP-03 · repo: server · agent: builder (claude-opus-5-5) · at: 2026-10-30T09:12:44Z · status: done

by Ada <ada@example.org> on feat/tidal-index

ref: https://tracker.example.org/browse/TP-03

### change

- `e-3w9k1d` Added the hourly tidal index table.

### decision

- `e-7h2m9q` [user] Index buckets are hourly, to match the sensor rate.

### open

- `e-5c8r0t` Should old buckets be pruned after 90 days?  (resolved by a-9x3b2c)

none: note, helper

#### addendum a-9x3b2c · builder (claude-opus-5-5) · 2026-10-31T08:02:10Z · status: done

- by Bob <bob@example.org> on fix/prune
- ref: https://tracker.example.org/browse/TP-03-prune
- `e-8k3m5p` Pruning was added in the nightly job.
- resolves: e-5c8r0t
```

For a project with local children (`cairn project link`), the Markdown has the notes of the current project exactly as above, then, for each child, a heading `# child: <name>` followed by that child's notes in the same layout. A child that cannot be read is skipped with a warning on standard error that starts with `cairn: ` and names the command to run (`cairn project link --project <parent> <child> <path>`); the exit code does not change.

A child's heading `# child: <name>` is printed even when the child has no matching notes (a child whose task pattern does not accept `--task` is skipped, with no heading). Children are read only, never written (parent to child only). `--task` is matched with the task pattern of each project on its own, and a project whose pattern does not accept it contributes nothing; the usage error (exit 2) applies only when no project accepts it. With `--out`, the line `exported N note(s)` counts the notes of all the projects read.

Markdown is for reading; it is not meant to be imported. The `by ...` line (empty parts left out) and the `ref:` line of a note appear after its metadata line, and only when set; an addendum shows them as list items under its heading. Only the `ref` stored on that note or addendum is printed, not a link computed from the project's ticket URL.

`--format json` stays on the current project: it never opens a child and never prints a child warning, so the file stays importable and faithful to that one database. With `--format json` the document is an object with `format` (always `cairn-export`), `version` (`1`), `project` (the project configuration), `notes` (full notes with their IDs) and `addenda` (full addenda with their IDs), indented by two spaces.

With `--out`, standard output shows one line: `exported 3 note(s) to /work/tidepool-notes.json`.

## Exit codes

- `0`: exported.
- `1`: runtime error: `--out` names an existing file and `--force` is not given; `--out` is a symlink or not a regular file; a write fails.
- `2`: usage error: a positional argument, `--format` not `md` or `json`, or `--task` does not match the project task pattern.
- `4`: no project found.

## Examples

```
cairn export --format json --out /work/tidepool-notes.json
```

Writes every note and addendum of the project to the file, mode 0600, and prints `exported 3 note(s) to /work/tidepool-notes.json`. Exit code 0.

```
cairn export --task TP-03 > tp-03.md
```

Writes the Markdown notes of TP-03 and its extensions to `tp-03.md`. Exit code 0.

```
cairn export --format json --out /work/tidepool-notes.json --force
```

Replaces the existing regular file with the new export. Exit code 0. Without `--force`, the same command exits 1.

## Notes

- `--format json` stays on the current project and never includes the notes of local children. To move a child's notes, run the export inside the child.
- Use `--format json` to move notes: `cairn import` accepts only that format.
- `--out` never follows a symbolic link. It replaces a file only with `--force`, and only when the existing path is a regular file.
- JSON notes and addenda carry `author_name`, `author_email`, `branch` and `ref` when they are set; `cairn import` restores them.
- The export contains every field of every note, including personal data that was stored in notes. Treat the file as sensitive.
- Export is read-only; it never changes the database.
