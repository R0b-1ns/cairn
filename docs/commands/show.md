# cairn show

Prints one note, addendum or entry in full, with nothing clipped.

## Usage

```
cairn show <id> [--project NAME] [--json]
```

## Arguments

| Argument | Required | Meaning |
|---|---|---|
| `<id>` | yes | `n-` + 6 characters for a note (`n-4k7q2m`), `a-` + 6 for an addendum (`a-9x3b2c`), `e-` + 6 for an entry (`e-7h2m9q`). Characters are lowercase `0-9` and `a-z` without `i`, `l`, `o`, `u`. |

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--project` | (empty) | a registered project name | Project to read. Default: found from the current directory, or env `CAIRN_PROJECT`. |
| `--json` | `false` | flag, no value | Print JSON instead of text. |

## Output

A note (`n-...`):

```
n-4k7q2m  Add tidal index
task: TP-03   repo: server   agent: builder   model: claude-opus-5-5   at: 2026-10-30T09:12:44Z
status: done
entries:
  e-3w9k1d  change   Added the hourly tidal index table.
  e-7h2m9q  decision [user] Index buckets are hourly, to match the sensor rate.
  e-5c8r0t  open     Should old buckets be pruned after 90 days?  (resolved by a-9x3b2c)
none: note, helper

addendum a-9x3b2c  2026-10-31T08:02:10Z  builder (claude-opus-5-5)  status: done
  e-8k3m5p  note     Pruning was added in the nightly job.
  resolves: e-5c8r0t
```

The `status:` line gains `(set when written: in_progress)` when the current status differs from the one written with the note. A note without task shows `task: -`. `impacts:` and `commits:` lines appear when set.

An addendum (`a-...`) prints `a-9x3b2c  addendum to note n-4k7q2m`, then its agent, model and time, its status and its entries.

An entry (`e-...`) prints its ID, type and location, then the entry line:

```
e-7h2m9q  decision  in note n-4k7q2m
e-7h2m9q  decision [user] Index buckets are hourly, to match the sensor rate.
```

`--json` for a note prints the note's fields (`id`, `agent`, `repo`, `at`, `task`, `model`, `title`, `status`, `entries`, `none`, `impacts`, `commits`, `resolves`, `supersedes`) plus `current_status` and `addenda` (the addenda, as objects with `id`, `note_id`, `agent`, `model`, `at`, `status`, `entries`, `resolves`, `supersedes`). For an addendum it prints the addendum object. For an entry it prints the entry object with `id`, `type`, `text`, `by`, `note_id`, `addendum_id`, `task`, `repo`, `at`, `closed`, `closed_by`, `closed_at`.

## Exit codes

- `0`: printed.
- `1`: runtime error, or the ID does not exist in the project (`n-4k7q2m not found`).
- `2`: usage error: not exactly one argument, or the argument is not a valid note, addendum or entry ID.
- `4`: no project found.

## Examples

```
cairn show n-4k7q2m
```

Prints the note, its entries, and its addenda, as in Output. Exit code 0.

```
cairn show e-7h2m9q
```

Prints the entry's location and its line. Exit code 0.

```
cairn show n-4k7q2m --json
```

Prints the note as one JSON object with `current_status` and `addenda`. Exit code 0.

```
cairn show n-zzzzzz
```

Exit code 1 if no such note exists in the project. `cairn show abc` exits 2: the argument is not a valid ID.

## Notes

- Use `show` after `list`, `brief`, `search` or `recent` to read the full text; those commands clip long text.
- An entry's state (`resolved by`, `superseded by`) is shown with the ID of the note or addendum that closed it.
- `show` never changes data.
