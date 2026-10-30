# cairn import

Imports a JSON export made by `cairn export --format json`. Every note is validated first. Existing data is never changed.

## Usage

```
cairn import <file|-> [--project NAME]
```

## Arguments

| Argument | Required | Meaning |
|---|---|---|
| `<file\|->` | yes | Path to the JSON export, or `-` to read it from standard input. At most 32 MiB. |

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--project` | (empty) | a registered project name | Project to write to. Default: found from the current directory, or env `CAIRN_PROJECT`. |

## Output

Success:

```
imported 3 notes and 4 addenda (0 already present)
```

Re-importing the same file:

```
imported 0 notes and 0 addenda (7 already present)
```

Validation failure prints on standard error, and nothing is imported:

```
cairn: invalid input, 1 field(s) to fix:
  notes[0].entries[1].id: is required in an import
```

A note or addendum whose ID already exists with different content refuses the whole import:

```
cairn: import refused, nothing was imported: ... already exists with different content
```

## Exit codes

- `0`: imported, or everything was already present.
- `1`: runtime error: the file cannot be opened, an ID conflicts with different stored content, or the database fails.
- `2`: usage error: not exactly one argument.
- `3`: invalid input: the file is not valid JSON, has unknown keys, is not a cairn export (`format` must be `cairn-export`, `version` must be `1`), is larger than 32 MiB, or a note or entry fails validation.
- `4`: no project found.

## Examples

```
cairn export --format json --out /work/tidepool-notes.json
cairn import /work/tidepool-notes.json --project tidepool
```

The first command writes the export of `tidepool`. The second imports it into a machine where `tidepool` holds none of these IDs, and prints `imported 3 notes and 4 addenda (0 already present)`. Every note is validated against the receiving project, so the repos and task IDs it uses must be valid there; the `project` object in the file is ignored.

```
cat /work/tidepool-notes.json | cairn import -
```

Reads the export from standard input. Exit code 0 when valid.

```
cairn import /work/tidepool-notes.json
```

Run a second time on the same data: prints `imported 0 notes and 0 addenda (7 already present)`. Exit code 0.

## Notes

- Every note and every entry must carry its `id` (`n-`, `a-`, `e-`). Hand-written files without IDs are refused with exit code 3.
- Identical items already present are skipped and counted as "already present". Items with the same ID but different content refuse the whole import (exit code 1); nothing is written.
- Every note must use a repo listed in the target project, and a task that matches its task pattern; otherwise exit code 3.
- The `unknown` model name is accepted on import only.
- Import never modifies the target project's configuration or any existing note.
- Input is checked completely before any write; the write is one transaction.
