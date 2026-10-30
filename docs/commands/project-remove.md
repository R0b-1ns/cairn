# cairn project remove

Unregisters a project from this machine. It never deletes the project's database or its folder.

## Usage

```
cairn project remove <name>
```

## Arguments

| Argument | Required | Meaning |
|---|---|---|
| `<name>` | yes | Registered project name, e.g. `tidepool`. |

## Flags

None. The command accepts no flags.

## Output

```
unregistered tidepool (its database was not deleted)
```

## Exit codes

- `0`: unregistered.
- `1`: runtime error: the name is not registered, or the registry cannot be written.
- `2`: usage error: not exactly one argument.

## Examples

```
cairn project remove tidepool
```

Prints `unregistered tidepool (its database was not deleted)`. Exit code 0. The notes remain in the database; `cairn projects` no longer lists `tidepool`.

```
cairn project remove tidepool
```

Run again: exit code 1, the name is not registered.

## Notes

- The database is never deleted. A project-mode database stays in `.cairn/` inside its folder; a machine-mode database stays in the data directory.
- After removal, commands that use `--project tidepool` or `CAIRN_PROJECT=tidepool` fail with exit code 4.
- To register a project-mode database again, run `cairn project add <path>`.
