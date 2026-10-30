# cairn guide

Prints the one-page usage guide for agents.

## Usage

```
cairn guide
```

## Arguments

None. A positional argument gives exit code 2.

## Flags

None. The command accepts no flags except `--help`, which prints the command's flags and exits 0.

## Output

Standard output is the complete text of the repository's `docs/guide.md`, unchanged. It starts with:

```
# cairn guide

`cairn` stores immutable work notes in SQLite and returns them in small slices.
```

The output does not depend on any project and does not read the database.

## Exit codes

- `0`: printed.
- `2`: usage error: an argument was given.

## Examples

```
cairn guide
```

Prints the whole guide. Exit code 0.

```
cairn guide | head -n 20
```

Prints the first 20 lines of the guide: the title, the summary and the routine.

## Notes

- Run `cairn guide` before using cairn for the first time in a session.
- The guide is a summary. For the flags of one command, run `cairn <command> --help`, which exits 0 and lists every flag with its default.
- The full reference is the `docs/` folder of the repository, one file per command under `docs/commands/`.
