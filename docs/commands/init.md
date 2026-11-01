# cairn init

Creates a project: its database, and its entry in the machine registry. Only `init` creates a database.

## Usage

```
cairn init --name NAME --repos REPO[,REPO...] [--mode project|machine] [--path DIR] [--context TEXT] [--default-repo REPO] [--tasks auto|jira|linear|github|custom] [--task-prefix LETTERS] [--task-url TEMPLATE] [--task-pattern REGEX] [--no-input]
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
| `--default-repo` | (empty; the only repo when `--repos` lists exactly one) | one of the names in `--repos` | Repo that `cairn add` uses when `--repo` is omitted. Stored as the project key `default_repo`; refused with exit 3 if it is not in `--repos`. |
| `--tasks` | (empty; see Task numbering) | `auto`, `jira`, `linear`, `github`, `custom` | How tasks are numbered. The mode compiles to the stored task pattern (see `configuration.md`). `custom` needs `--task-pattern`. An unknown value is refused with exit 3, listing the valid modes. Cannot be combined with `--task-pattern` (exit 2). |
| `--task-prefix` | (empty; `T` for `auto`) | 1-5 capital letters (`^[A-Z]{1,5}$`) | Prefix of the `auto` mode: `FEAT` gives `FEAT-1`, `FEAT-2`. Refused with exit 3 with any other mode. |
| `--task-url` | (empty) | an `http://` or `https://` URL containing `{task}`, up to 500 characters | Template that turns a task into a link; `{task}` is replaced by the task ID. Allowed with every mode. |
| `--task-pattern` | (empty; when no task flag is given the pattern is `^(?P<base>[A-Z]{2,5}-\d{1,4})(?:\.(?P<ext>\d{1,3}))?$`) | a regular expression with a named group `(?P<base>...)` and optionally `(?P<ext>...)`; at most 200 characters | The format every task ID must match. Given alone, it means `--tasks custom` with this pattern. |
| `--no-input` | `false` | | Do not ask questions even in a terminal. |

## Task numbering

| Mode | Task IDs | Example |
|---|---|---|
| `auto` | `PREFIX-n`, prefix from `--task-prefix` (default `T`) | `T-1`, `T-2.1` |
| `jira` | Jira keys | `PROJ-123` |
| `linear` | Linear IDs | `ENG-42` |
| `github` | issue numbers | `#128` |
| `custom` | your own regular expression, from `--task-pattern` | `TP-03` |

With none of `--tasks` and `--task-pattern`, `init` keeps the legacy default: mode `custom` with the pattern `^(?P<base>[A-Z]{2,5}-\d{1,4})(?:\.(?P<ext>\d{1,3}))?$`, and writes one line on standard error: `cairn: note: task numbering was not chosen; using the default TP-03 format (see --tasks)`. The project is still created and the exit code is 0. Existing projects are not changed.

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
- `2`: usage error: a positional argument was given, or `--tasks` was combined with `--task-pattern`.
- `3`: invalid input: name, mode, repos, context, task mode, prefix, URL template or task pattern fails validation, including `--tasks custom` without `--task-pattern` and `--task-prefix` with a mode other than `auto`. Every bad field is listed on standard error.

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
cairn init --name tidepool --repos server --tasks auto --task-prefix FEAT
```

Creates a project whose tasks are numbered `FEAT-1`, `FEAT-2`, ... Exit code 0.

```
cairn init --name tidepool --repos server --tasks jira --task-url 'https://example.atlassian.net/browse/{task}'
```

Creates a project that accepts Jira keys such as `PROJ-123` and can link them. Exit code 0.

```
cairn init --name tidepool --repos server --tasks jira --task-pattern '^TP-\d+$'
```

Exit code 2: `--tasks` and `--task-pattern` cannot be combined.

```
cairn init --name Tidepool --repos server
```

Exit code 3: the name has an uppercase letter.

## Notes

- After a successful `init`, cairn asks GitHub for the latest release (3 seconds at most). If it differs from the installed version, one line goes to standard error: `cairn: a new version is available (1.2.0, installed: 1.0.0); install it with 'cairn update'`. Standard output, the exit code and the project are not affected, and any network problem is silent. Set `CAIRN_NO_UPDATE_CHECK` to any non-empty value to skip the check; development builds never check.
- `init` creates no notes. Start with `cairn add`.
- `--repos` is required. The names given here are the only values that `cairn add --repo` and `--impact` accept for this project.
- The legacy default task pattern accepts `TP-03`, `TP-03.1` and `ABCDE-1234`. Choose `--tasks` or pass `--task-pattern` when the task IDs differ.
- `.cairn/` in project mode lives inside the folder; `machine` mode keeps the folder clean.
- A project name or folder that is already registered is refused with exit code 1; `cairn projects` lists them.
