# cairn brief

Prints one merged view of a task: its notes, its changes, the decisions in force and the open points still unresolved.

## Usage

```
cairn brief <task-id> [--branch BRANCH] [--author AUTHOR] [--project NAME] [--json] [--max N]
```

## Arguments

| Argument | Required | Meaning |
|---|---|---|
| `<task-id>` | yes | A task ID matching the project task pattern, e.g. `TP-03`. A base task includes its extensions (`TP-03.1`, `TP-03.2`). An extension such as `TP-03.1` matches only itself. |

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--branch` | (empty) | a branch name | Only notes written on this branch; a note matches when it or one of its addenda was written there. Also narrows the `changes`, `decisions` and `open` lists to those notes. |
| `--author` | (empty) | an author name or email | Only notes by this author, matched exactly against the name or the email of the note or of one of its addenda. Also narrows the `changes`, `decisions` and `open` lists. |
| `--project` | (empty) | a registered project name | Project to read. Default: found from the current directory, or env `CAIRN_PROJECT`. |
| `--json` | `false` | flag, no value | Print JSON instead of text. `--max` does not apply to JSON. |
| `--max` | `50` | integer, 1 or more | Maximum lines printed per section. The rest is counted in a line `... N more (raise --max)`. |

## Output

Text, for a task with notes:

```
TP-03: 2 note(s)
refs: https://tracker.example.org/browse/TP-03
notes:
  n-4k7q2m  TP-03    done        2026-10-30  Add tidal index (server, builder) by Ada on feat/tidal-index
  n-9x2c7d  TP-03.1  in_progress 2026-10-31  Split the sensor feed (client, builder)
changes:
  e-3w9k1d  Added the hourly tidal index table.  (n-4k7q2m)
  e-6t4b2g  Split the sensor feed into two streams.  (n-9x2c7d)
decisions in force:
  e-7h2m9q  [user] Index buckets are hourly, to match the sensor rate.  (n-4k7q2m)
open points:
  e-5c8r0t  Should old buckets be pruned after 90 days?  (n-4k7q2m)
```

The `refs:` line lists every `ref` stored on the notes and addenda of the task and its extensions, newest first, separated by `, `. When none is stored and the project has a ticket URL (set at `init` with `--task-url`), it shows the link computed from the task instead. With neither, the line is not printed. A note line ends with a compact origin, ` by Name on branch`, when the note has an author or a branch: the name only, or the email when there is no name (` by ada@example.org on main` when only the email is stored), never both, to keep the output short; `show` and `export` print the full `by Name <email> on branch`. Empty parts are left out, and a note with neither adds nothing.

Local children: when the project has local children (`cairn project link`), the notes, changes, decisions in force and open points of each child are merged after those of the current project, and each line from a child ends with `  [<child>]` (after the origin text on note lines). A child is read, never written: the direction is parent to child only, so a child project never shows its parent's notes. The task ID is matched with the task pattern of each project on its own; a project whose pattern does not accept it contributes nothing and prints nothing, and the usage error (exit 2) applies only when no project accepts it. `--max` caps each section after merging, and the `... N more` count includes child items. A child that cannot be read is skipped with a warning on standard error that starts with `cairn: ` and names the command to run (`cairn project link --project <parent> <child> <path>`); the exit code does not change.

The `refs:` line (and the `refs` array) merges the refs of the current project and of its children without duplicates.

Sections are in this order: `notes` (oldest first), `changes` (oldest first), `decisions in force`, `open points`. An empty section prints `  none`. A change or open point taken from an addendum shows `(n-4k7q2m via a-9x3b2c)`. A task without notes prints `no notes for task TP-09`.

`--json` prints one object with the keys `task`, `notes`, `changes`, `decisions`, `open`, and the totals `notes_total`, `changes_total`, `decisions_total`, `open_total`, plus `refs` when there is at least one: an array of the same links as the `refs:` line (every stored `ref` of the task, newest first, each once, or the computed link when none is stored). `refs` is omitted when empty. Each list is capped by `--max`; the totals count everything that matched, so a consumer can tell when a list was cut.

- `notes`: objects with the keys `id`, `task`, `repo`, `agent`, `model`, `at`, `title`, `status` (the current status), and, when not empty, `branch`, `author_name`, `author_email`, `ref`.
- `changes`, `decisions`, `open`: entry objects with `id`, `type`, `text`, `by`, `note_id`, `addendum_id`, `task`, `repo`, `at`, `closed`, `closed_by`, `closed_at`.

Empty lists are `[]`, never `null`. Items from a local child carry a `child` key with the child name; items of the current project have no `child` key. When the project has at least one link, the object also has `missing_children`: an array of the names of the local children that could not be read (`[]` when all were read). A project without links prints no `missing_children` key.

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
cairn brief TP-03 --branch fix/prune
```

Prints the merged view limited to the notes of TP-03 written (or amended) on `fix/prune`. Exit code 0.

```
cairn brief TP-09
```

Prints `no notes for task TP-09`. Exit code 0.

## Notes

- Use `brief` as the first read for a task. Use `show` for the full text of an item and `list` for filtered selections.
- Resolved open points and superseded decisions are hidden. Changes are never hidden.
- Text is clipped to 160 characters per line in text output. `cairn show <id>` prints the full text.
- A base task such as `TP-03` also includes `TP-03.1`; `TP-03.1` alone shows only its own notes.
