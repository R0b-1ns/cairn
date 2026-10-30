# cairn recent

Lists the last N subjects handled, most recently active first. A subject is a task (by its base ID) or, for a note without a task, that note.

## Usage

```
cairn recent [N] [--project NAME] [--json]
```

## Arguments

| Argument | Required | Meaning |
|---|---|---|
| `[N]` | no | Number of subjects to list: a positional integer, 1 or more. Default `10`. |

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--project` | (empty) | a registered project name | Project to read. Default: found from the current directory, or env `CAIRN_PROJECT`. |
| `--json` | `false` | flag, no value | Print a JSON array instead of lines. |

## Output

One line per subject: subject, day of the latest activity, status, title, number of notes.

```
TP-03  2026-10-30  in_progress Add tidal index  (3 notes)
TP-02  2026-10-29  done        Split the sensor feed  (1 note)
n-8r4w2e  2026-10-28  done        Rename the client build script  (1 note)
```

Activity is a note or an addendum. The status and title are those of the subject's most recently active note. With no notes the output is `no notes yet`.

`--json` prints an array of objects with the keys `subject`, `last_at` (RFC3339 UTC), `status`, `title`, `note_id` (the most recently active note) and `notes` (number of notes in the subject). An empty result is `[]`.

## Exit codes

- `0`: listed.
- `1`: runtime error.
- `2`: usage error: more than one argument, or `N` is not a positive integer.
- `4`: no project found.

## Examples

```
cairn recent
```

Prints the 10 most recently active subjects, as in Output. Exit code 0.

```
cairn recent 3 --json
```

Prints a JSON array of the 3 most recently active subjects. Exit code 0.

```
cairn recent 0
```

Exit code 2: `N must be a positive integer`.

## Notes

- Use `recent` to find where to resume work, then `cairn brief <task>` on the subject.
- A task's extensions (`TP-03.1`) are counted under their base subject `TP-03`.
- `recent` has no `--max` flag; `N` is the limit.
