# cairn init

Creates a project: its database, and its entry in the machine registry. Only `init` creates a database.

## Usage

```
cairn init --name NAME --repos REPO[,REPO...] [--mode project|machine] [--path DIR] [--context TEXT] [--task-pattern REGEX]
```

## Arguments

None. A positional argument gives exit code 2.

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--name` | (empty) | lowercase letters, digits, `-`, `_`; starts with a letter or digit; 1-40 characters | Project name. Required. Must be unique on this machine. |
| `--mode` | `project` | `project`, `machine` | `project`: database in `.cairn/cairn.db` inside the folder. `machine`: database in the data directory (`CAIRN_HOME`), outside the folder. |
| `--path` | (empty; the current directory) | an existing folder | Project root folder. |
| `--context` | (empty) | text, up to 2000 characters, newlines allowed | A few lines describing the project. Shown by `cairn context`. |
| `--repos` | (empty) | comma-separated names; each lowercase letters, digits, `.`, `-`, `_`, 1-40 characters; at most 50 | Allowed repo names. Required, at least one. |
| `--task-pattern` | `^(?P<base>[A-Z]{2,5}-\d{1,4})(?:\.(?P<ext>\d{1,3}))?$` | a regular expression with a named group `(?P<base>...)` and optionally `(?P<ext>...)`; at most 200 characters | The format every task ID must match. |

`init` has no `--project` and no `--json` flag: it names the project itself.

## Output

Project mode:

```
initialized project tidepool (project mode) at /work/tidepool
database: /work/tidepool/.cairn/cairn.db
```

Machine mode prints `(machine mode)` and the database path under the data directory, `<data directory>/projects/tidepool/cairn.db`.

## Exit codes

- `0`: project created and registered.
- `1`: runtime error: the name is already registered, the folder is already registered, the folder already holds a `.cairn` store, `--path` is not a folder, or a file cannot be written.
- `2`: usage error: a positional argument was given.
- `3`: invalid input: name, mode, repos, context or task pattern fails validation. Every bad field is listed on standard error.

## Examples

```
cd /work/tidepool && cairn init --name tidepool --repos server,client --context "Tide data service."
```

Creates `/work/tidepool/.cairn/cairn.db` and prints the two lines in Output. Exit code 0.

```
cairn init --name tidepool --repos server,client --mode machine --path /work/tidepool
```

Creates the database under the data directory and registers `tidepool` at `/work/tidepool`, on a machine where `tidepool` is not yet registered. Exit code 0.

```
cairn init --name tidepool --repos server --task-pattern '^(?P<base>TP-\d{2})(?:\.(?P<ext>\d{1,3}))?$'
```

Creates a project whose task IDs are `TP-` followed by exactly two digits, with optional `.n` extensions. Exit code 0.

```
cairn init --name Tidepool --repos server
```

Exit code 3: the name has an uppercase letter.

## Notes

- `init` creates no notes. Start with `cairn add`.
- `--repos` is required. The names given here are the only values that `cairn add --repo` and `--impact` accept for this project.
- The default task pattern accepts `TP-03`, `TP-03.1` and `ABCDE-1234`. Pass `--task-pattern` only when the task IDs differ.
- `.cairn/` in project mode lives inside the folder; `machine` mode keeps the folder clean.
- A project name or folder that is already registered is refused with exit code 1; `cairn projects` lists them.
