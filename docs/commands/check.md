# cairn check

Reports done tasks that have no note, done tasks whose latest note is not done, and open points older than a number of days. Exits 5 when anything is reported.

## Usage

```
cairn check [--done TASKS] [--done-file FILE] [--older-than DAYS] [--max N] [--project NAME] [--json]
```

## Arguments

None. A positional argument gives exit code 2.

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--done` | (empty) | comma-separated task IDs, e.g. `TP-01,TP-02` | Task IDs the caller considers done. |
| `--done-file` | (empty) | path to a file, or `-` for standard input | File with one task ID per line. Blank lines are ignored; `#` starts a comment. Merged with `--done`. |
| `--older-than` | `14` | integer, 0 or more | Report unresolved open points older than this many days. |
| `--max` | `50` | integer, 1 or more | Maximum open points listed. The rest is counted. |
| `--project` | (empty) | a registered project name | Project to read. Default: found from the current directory, or env `CAIRN_PROJECT`. |
| `--json` | `false` | flag, no value | Print JSON instead of text. |

## Output

Text, when there are findings:

```
done tasks without notes:
  TP-04  no note for this task
done tasks whose latest note is not done:
  TP-02  latest note n-6p1x8w is in_progress
open points older than 14 days:
  e-5c8r0t  2026-09-02  Should old buckets be pruned after 90 days?  (n-4k7q2m)
  ... 3 more (raise --max)
```

Sections without findings are left out. With no findings, the output is `ok: nothing to report`.

`--json` prints one object:

```json
{
  "missing_notes": [{"task": "TP-04", "detail": "no note for this task"}],
  "not_done": [{"task": "TP-02", "detail": "latest note n-6p1x8w is in_progress"}],
  "old_open": [{"id": "e-5c8r0t", "type": "open", "text": "Should old buckets be pruned after 90 days?", "by": "", "note_id": "n-4k7q2m", "addendum_id": "", "task": "TP-03", "repo": "server", "at": "2026-09-02T08:14:00Z", "closed": false, "closed_by": "", "closed_at": ""}],
  "ok": false
}
```

`old_open` holds entry objects (same keys as `cairn list --json`). `ok` is `true` only when all three lists are empty. Empty lists are `[]`.

## Exit codes

- `0`: nothing reported (`ok: nothing to report`).
- `1`: runtime error, including a `--done-file` that cannot be read.
- `2`: usage error: a positional argument, `--older-than` below 0, `--max` below 1, or a task ID that does not match the project task pattern.
- `4`: no project found.
- `5`: at least one finding. Only the exit code is set; output is still printed.

## Examples

```
cairn check --done TP-01,TP-02,TP-04
```

Prints the `done tasks without notes` section for TP-04 and the `not done` section for TP-02, then exits 5.

```
printf 'TP-01\nTP-03 # shipped in the last release\n' | cairn check --done-file -
echo $?
```

Prints `ok: nothing to report` when TP-01 and TP-03 each have a done latest note, then `0`.

```
cairn check --older-than 30 --json
```

Lists open points older than 30 days as JSON. Exit code 5 when `old_open` is not empty.

## Notes

- Without `--done` or `--done-file`, only old open points are checked.
- A task is "without notes" when no note carries its ID. A task is "not done" when its most recent note (including its extensions for a base task) has a status other than `done`.
- Open points count as old by the time of the note or addendum that holds them. Resolved open points are not checked.
- Exit code 5 is a finding, not an error: scripts can branch on it.
- Combine with `--done-file` for long lists; the file is read at most 1 MiB.
