# cairn project add

Registers a project-mode database that already exists in a folder, for example one copied from another machine, so that cairn can find it by name.

## Usage

```
cairn project add <path>
```

## Arguments

| Argument | Required | Meaning |
|---|---|---|
| `<path>` | yes | Project root folder. It must contain `.cairn/cairn.db`, a project-mode database. |

## Flags

None. The command accepts no flags.

## Output

```
registered tidepool at /work/tidepool
```

The name is the project name stored inside the database. The path is stored as an absolute path with symlinks resolved.

## Exit codes

- `0`: registered.
- `1`: runtime error: no `.cairn/cairn.db` at the path; the database is not in project mode; the name or the folder is already registered; the registry cannot be written.
- `2`: usage error: not exactly one argument.

## Examples

```
cairn project add /work/tidepool
```

Registers `tidepool` at `/work/tidepool` and prints `registered tidepool at /work/tidepool`. Exit code 0.

```
cairn project add /work/tidepool
```

Run a second time: exit code 1, because the folder is already registered.

## Notes

- Only project-mode databases can be added. A machine-mode database lives in the data directory and is registered by `cairn init --mode machine`.
- `project add` does not change the database; it only writes the registry.
- Use `cairn projects` to see what is registered.
