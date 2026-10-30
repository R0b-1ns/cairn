# cairn brief

Prints one merged view of a task: its notes, its changes, the decisions in force and the open points still unresolved.

## Usage

```
cairn brief <task-id> [--project NAME] [--json] [--max N]
```

## Arguments

| Argument | Required | Meaning |
|---|---|---|
| `<task-id>` | yes | A task ID matching the project task pattern, e.g. `TP-03`. A base task includes its extensions (`TP-03.1`, `TP-03.2`). An extension such as `TP-03.1` matches only itself. |

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--project` | (empty) | a registered project name | Project to read. Default: found from the current directory, or env `CAIRN_PROJECT`. |
| `--json` | `false` | flag, no value | Print JSON instead of text. `--max` does not apply to JSON. |
| `--max` | `50` | integer, 1 or more | Maximum lines printed per section. The rest is counted in a line `... N more (raise --max)`. |

## Output

Text, for a task with notes:

```
TP-03: 2 note(s)
notes:
  n-4k7q2m  TP-03    done        2026-10-30  Add tidal index (server, builder)
  n-9x2c7d  TP-03.1  in_progress 2026-10-31  Split the sensor feed (client, builder)
changes:
  e-3w9k1d  Added the hourly tidal index table.  (n-4k7q2m)
  e-6t4b2g  Split the sensor feed into two streams.  (n-9x2c7d)
decisions in force:
  e-7h2m9q  [user] Index buckets are hourly, to match the sensor rate.  (n-4k7q2m)
open points:
  e-5c8r0t  Should old buckets be pruned after 90 days?  (n-4k7q2m)
```

Sections are in this order: `notes` (oldest first), `changes` (oldest first), `decisions in force`, `open points`. An empty section prints `  none`. A change or open point taken from an addendum shows `(n-4k7q2m via a-9x3b2c)`. A task without notes prints `no notes for task TP-09`.

`--json` prints one object with the keys `task`, `notes`, `changes`, `decisions`, `open`, and the totals `notes_total`, `changes_total`, `decisions_total`, `open_total`. Each list is capped by `--max`; the totals count everything that matched, so a consumer can tell when a list was cut.

- `notes`: objects with the keys `id`, `task`, `repo`, `agent`, `model`, `at`, `title`, `status` (the current status).
- `changes`, `decisions`, `open`: entry objects with `id`, `type`, `text`, `by`, `note_id`, `addendum_id`, `task`, `repo`, `at`, `closed`, `closed_by`, `closed_at`.

Empty lists are `[]`, never `null`.

## Exit codes

- `0`: printed (including "no notes for task").
- `1`: runtime error.
- `2`: usage error: not exactly one argument, `--max` below 1, or the task ID does not match the project task pattern.
- `4`: no project found.

## Examples

```
cairn brief TP-03
```

Prints the merged view of TP-03 and its extensions, as in Output. Exit code 0. Read this before starting work on a task.

```
cairn brief TP-03 --json
```

Prints the JSON object with `task` set to `TP-03` and the four lists. Exit code 0.

```
cairn brief TP-09
```

Prints `no notes for task TP-09`. Exit code 0.

## Notes

- Use `brief` as the first read for a task. Use `show` for the full text of an item and `list` for filtered selections.
- Resolved open points and superseded decisions are hidden. Changes are never hidden.
- Text is clipped to 160 characters per line in text output. `cairn show <id>` prints the full text.
- A base task such as `TP-03` also includes `TP-03.1`; `TP-03.1` alone shows only its own notes.
