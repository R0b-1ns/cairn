# cairn list

Lists entries (changes, decisions, open points, notes, helpers), newest first, with combinable filters.

## Usage

```
cairn list [--type TYPE] [--task TASK] [--repo REPO] [--impacts REPO] [--since DATE] [--until DATE] [--resolved | --all] [--max N] [--project NAME] [--json]
```

## Arguments

None. A positional argument gives exit code 2.

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--type` | (empty) | `change`, `decision`, `open`, `note`, `helper` | Only entries of this type. |
| `--task` | (empty) | a task ID matching the project task pattern | Only entries of this task. A base task includes its extensions; an extension matches exactly. |
| `--repo` | (empty) | a repo of the project | Only entries of notes written in this repo. |
| `--impacts` | (empty) | a repo of the project | Only entries of notes that list this repo in their impacts. |
| `--since` | (empty) | `YYYY-MM-DD` or RFC3339 | Only entries at or after this time. A date means 00:00 UTC. |
| `--until` | (empty) | `YYYY-MM-DD` or RFC3339 | Only entries up to and including this time. A date includes the whole day. |
| `--resolved` | `false` | flag, no value | Show only resolved open points and superseded decisions, with what closed them. Cannot be combined with `--all`. |
| `--all` | `false` | flag, no value | Show closed and open entries. Cannot be combined with `--resolved`. |
| `--max` | `50` | integer, 1 or more | Maximum entries printed. The rest is counted in `... N more (raise --max)`. |
| `--project` | (empty) | a registered project name | Project to read. Default: found from the current directory, or env `CAIRN_PROJECT`. |
| `--json` | `false` | flag, no value | Print a JSON array instead of lines. |

Without `--resolved` or `--all`, resolved open points and superseded decisions are hidden.

## Output

One line per entry: ID, type, task, repo, day, decision author, text, state, note ID.

```
e-7h2m9q  decision TP-03  server  2026-10-30  [user] Index buckets are hourly, to match the sensor rate.  n-4k7q2m
e-3w9k1d  change   TP-03  server  2026-10-30  Added the hourly tidal index table.  n-4k7q2m
e-5c8r0t  open     TP-03  server  2026-10-30  Should old buckets be pruned after 90 days?  n-4k7q2m
... 12 more (raise --max)
```

With `--resolved` or `--all`, a closed entry carries its closer:

```
e-2w7n4k  open     TP-02  client  2026-10-20  Keep the old sensor schema?  (resolved by a-9x3b2c)  n-6p1x8w
```

Decisions superseded by a later note show `(superseded by ...)`. A note without task shows `-` in the task column. With no match the output is `no entries`.

`--json` prints an array of entry objects with the keys `id`, `type`, `text`, `by`, `note_id`, `addendum_id`, `task`, `repo`, `at`, `closed`, `closed_by`, `closed_at`. `--max` applies. An empty result is `[]`.

## Exit codes

- `0`: printed (including `no entries`).
- `1`: runtime error.
- `2`: usage error: a positional argument; `--max` below 1; `--resolved` with `--all`; `--type` not one of the allowed values; `--since` or `--until` not a valid date; `--task` not matching the project task pattern.
- `4`: no project found.

## Examples

```
cairn list --type open --task TP-03
```

Prints the open points of TP-03 that are not resolved, newest first. Exit code 0.

```
cairn list --since 2026-10-01 --until 2026-10-31 --repo client --max 20
```

Prints up to 20 entries of notes written in `client` during October 2026. Exit code 0.

```
cairn list --resolved --type open --json
```

Prints a JSON array of resolved open points; each `closed` is `true` and `closed_by` is the ID of the closing note or addendum. Exit code 0.

## Notes

- Text output clips each entry to 160 characters. Use `cairn show <id>` for the full text.
- `list` shows the note ID but not the addendum ID. Use `show` on the entry to see where it was written.
- `--since` and `--until` compare the time of the note or addendum that holds the entry.
- To find a superseded decision or a resolved open point, use `--resolved`; to see everything, use `--all`.
