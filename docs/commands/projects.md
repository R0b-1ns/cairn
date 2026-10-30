# cairn projects

Lists every project registered on this machine.

## Usage

```
cairn projects [--json]
```

## Arguments

None. A positional argument gives exit code 2.

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--json` | `false` | flag, no value | Print a JSON array instead of a table. |

## Output

Text: a table with the columns `NAME`, `MODE`, `NOTES`, `PATH`.

```
NAME      MODE     NOTES  PATH
tidepool  project  12     /work/tidepool
```

- `MODE` is `project` or `machine`.
- `NOTES` is `?` when the database cannot be read.
- `[missing]` is appended to the path when that folder no longer exists.
- With no registered project the output is `no projects registered`.

`--json` prints an array of objects with the keys `name`, `mode` (`project` or `machine`), `path`, `notes` (number of notes, or `-1` when unreadable) and `missing` (`true` or `false`). An empty registry is `[]`.

## Exit codes

- `0`: listed.
- `1`: runtime error: the registry cannot be read.
- `2`: usage error: a positional argument was given.

## Examples

```
cairn projects
```

Prints the table shown in Output. Exit code 0.

```
cairn projects --json
```

Prints, for example, `[{"name":"tidepool","mode":"project","path":"/work/tidepool","notes":12,"missing":false}]`, indented. Exit code 0.

## Notes

- This command needs no current project; it works from any folder.
- The registry is the list that `--project NAME` and `CAIRN_PROJECT` resolve against.
- Use `cairn project move` to fix a `[missing]` path, or `cairn project remove` to unregister it.
