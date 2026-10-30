# cairn project move

Updates the registered folder of a project after the folder has been moved or renamed on disk.

## Usage

```
cairn project move <name> <new-path>
```

## Arguments

| Argument | Required | Meaning |
|---|---|---|
| `<name>` | yes | Registered project name, e.g. `tidepool`. |
| `<new-path>` | yes | The new project root folder. It must exist and be a folder. For a project-mode project it must contain `.cairn/cairn.db`. |

## Flags

None. The command accepts no flags.

## Output

```
moved tidepool to /work/tidepool-moved
```

The files are not moved or copied. Only the registry is updated.

## Exit codes

- `0`: moved.
- `1`: runtime error: the name is not registered; the new path is not an existing folder; a project-mode project has no `.cairn/cairn.db` at the new path; the registry cannot be written.
- `2`: usage error: not exactly two arguments.

## Examples

```
mv /work/tidepool /work/tidepool-moved && cairn project move tidepool /work/tidepool-moved
```

Moves the folder, then records the new path. Prints `moved tidepool to /work/tidepool-moved`. Exit code 0.

```
cairn project move tidepool /work/missing
```

Exit code 1 when `/work/missing` does not exist.

## Notes

- Run this after moving the folder; cairn cannot find a project whose registered path no longer exists. `cairn projects` shows `[missing]` for such entries.
- The command does not check that the new folder holds the same project. Use `cairn context` inside the folder to confirm the name.
