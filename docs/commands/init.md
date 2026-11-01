# cairn init

Creates a project: its database, and its entry in the machine registry. Only `init` creates a database.

## Usage

```
cairn init --name NAME --repos REPO[,REPO...] [--mode project|machine] [--path DIR] [--context TEXT] [--default-repo REPO] [--tasks auto|jira|linear|github|custom] [--task-prefix LETTERS] [--task-url TEMPLATE] [--task-pattern REGEX] [--no-input]
cairn init --project PARENT --repo REPO [--local [--name CHILD] [--context TEXT]] [--path DIR] [--no-input]
```

## Arguments

None. A positional argument gives exit code 2.

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--name` | (empty) | lowercase letters, digits, `-`, `_`; starts with a letter or digit; 1-40 characters | Project name. Required. Must be unique on this machine. With `--project`: only with `--local` (default: the repo name, which must then be a valid project name, so a repo such as `web.app` needs `--name`). |
| `--mode` | `project` | `project`, `machine` | `project`: database in `.cairn/cairn.db` inside the folder. `machine`: database in the data directory (`CAIRN_HOME`), outside the folder. |
| `--path` | (empty; the current directory) | an existing folder | Project root folder. |
| `--context` | (empty) | text, up to 2000 characters, newlines allowed | A few lines describing the project. Shown by `cairn context`. With `--project` only with `--local` (exit 2 otherwise: an attached child has no database of its own). |
| `--repos` | (empty) | comma-separated names; each lowercase letters, digits, `.`, `-`, `_`, 1-40 characters; at most 50 | Allowed repo names. Required, at least one. |
| `--default-repo` | (empty; the only repo when `--repos` lists exactly one) | one of the names in `--repos` | Repo that `cairn add` uses when `--repo` is omitted. Stored as the project key `default_repo`; refused with exit 3 if it is not in `--repos`. |
| `--tasks` | (empty; see Task numbering) | `auto`, `jira`, `linear`, `github`, `custom` | How tasks are numbered. The mode compiles to the stored task pattern (see `configuration.md`). `custom` needs `--task-pattern` (exit 3 without it). The other modes cannot be combined with `--task-pattern` (exit 2). An unknown value is refused with exit 3, listing the valid modes. |
| `--task-prefix` | (empty; `T` for `auto`) | 1-5 capital letters (`^[A-Z]{1,5}$`) | Prefix of the `auto` mode: `FEAT` gives `FEAT-1`, `FEAT-2`. Refused with exit 3 with any other mode, and when given alone without `--tasks auto`. |
| `--task-url` | (empty) | an `http://` or `https://` URL containing `{task}`, up to 500 characters | Template that turns a task into a link; `{task}` is replaced by the base task, without a leading `#` and without its extension (`PROJ-12.1` and `PROJ-12` both give `PROJ-12`, `#128` gives `128`), escaped as one URL path segment. Allowed with every mode. |
| `--task-pattern` | (empty; when no task flag is given the pattern is `^(?P<base>[A-Z]{2,5}-\d{1,4})(?:\.(?P<ext>\d{1,3}))?$`) | a regular expression with a named group `(?P<base>...)` and optionally `(?P<ext>...)`; at most 200 characters | The format every task ID must match. Given alone, it means `--tasks custom` with this pattern; it also goes with `--tasks custom`, and with no other mode (exit 2). An empty value is refused with exit 3. |
| `--project` | (empty) | the name of a project registered on this machine | Join that project as a child instead of creating a new one. Needs `--repo`. See Joining a project. |
| `--repo` | (empty) | one repo name (same rules as `--repos`) | With `--project`: the repo name of this folder. It is added to the parent's repos when missing. |
| `--local` | `false` | | With `--project`: give the child its own database in `.cairn/cairn.db` instead of attaching it to the parent's. Refused with exit 2 without `--project`. |
| `--no-input` | `false` | | Never ask questions, even in a terminal; every value comes from the flags or from the defaults. See Interactive questions. |

## Task numbering

| Mode | Task IDs | Example |
|---|---|---|
| `auto` | `PREFIX-n`, prefix from `--task-prefix` (default `T`) | `T-1`, `T-2.1` |
| `jira` | Jira keys | `PROJ-123` |
| `linear` | Linear IDs | `ENG-42` |
| `github` | issue numbers | `#128` |
| `custom` | your own regular expression, from `--task-pattern` | `TP-03` |

With none of `--tasks` and `--task-pattern` (and without questions, see below), `init` keeps the legacy default: mode `custom` with the pattern `^(?P<base>[A-Z]{2,5}-\d{1,4})(?:\.(?P<ext>\d{1,3}))?$`, and writes one line on standard error: `cairn: note: task numbering was not chosen; using the default TP-03 format (see --tasks)`. The project is still created and the exit code is 0. Existing projects are not changed.

`init` has no `--json` flag. Its `--project` flag does not select the project to run in (as other commands do): it names the parent to join.

## Joining a project

`cairn init --project tidepool --repo web` makes the current folder a child of the registered project `tidepool`.

- Attached (the default): the folder gets `.cairn/links.json` (pointer: parent name, parent id, repo; no path) and no database. Commands run inside it use the parent's database, and `cairn add` uses `--repo web` by default. The child is named after its repo, so a dotted repo such as `web.app` is allowed.
- Local (`--local`): the folder gets its own database with the single repo `--repo` (also its default repo), the parent's task settings (mode, pattern, prefix, URL template) and the parent recorded. It is registered as its own project, named `--name` (default: the repo name).
- `--name` is allowed only with `--local` (exit 2 otherwise).
- It never asks questions, even in a terminal; `--no-input` is accepted and changes nothing.
- It cannot be combined with `--repos`, `--mode`, `--default-repo`, `--tasks`, `--task-pattern`, `--task-prefix` or `--task-url` (exit 2): the child takes the parent's task settings and its own repo.
- Refused with exit 1: the parent is not registered (run `cairn project add <path>` for it), the parent is itself a child or the folder is already a parent (nesting is not supported), the parent already has an active child of that name, or the folder already holds a database or a pointer. A repo that fails validation, or a local child project name that does, gives exit 3.
- The link is recorded in the parent's database. For a project-mode parent that file changes and must be committed; a machine-mode parent prints no such request. If a step fails, the files this call created are removed and the registry is restored; the parent records the link as detached. The parent's `repo_added` and project id events stay (the event log is insert-only). If even the detach cannot be recorded, a warning names `cairn project unlink`.

## Interactive questions

When standard input and standard error are both terminals and `--no-input` is not given, `init` asks the questions whose flag was not given, in this order. Every question shows its default in brackets; Enter (or a line of spaces) accepts it. An invalid answer is explained and the question is asked again. Prompts go to standard error; standard output is unchanged.

| # | Question | Default | Asked when |
|---|---|---|---|
| 1 | Project name | the lowercased name of the folder, when it is a valid name; otherwise none (an answer is required) | `--name` not given |
| 2 | Storage mode: `project` (database in `.cairn/`) or `machine` (database in the data directory); a number or a name | `project` | `--mode` not given |
| 3 | Project root folder | the current directory | `--path` not given |
| 4 | Repos, comma-separated | the project name when the root folder holds a `.git` entry; otherwise none (an answer is required) | `--repos` not given |
| 5 | Default repo | none (Enter for none) | `--default-repo` not given and there are several repos |
| 6 | Context, one line | empty | `--context` not given |
| 7 | Task numbering: `auto`, `jira`, `linear`, `github` or `custom`, each shown with what it is for | `auto` | no task flag at all |
| 8 | Tag: 1-5 capital letters (`T` gives `T-1`, `FEAT` gives `FEAT-1`) | `T` | mode `auto`, `--task-prefix` not given |
| 9 | Task format: a regular expression with a named group `(?P<base>...)` | none (an answer is required) | mode `custom` chosen or given by `--tasks custom` without `--task-pattern` |
| 10 | Ticket URL template containing `{task}`, with an example shown | empty (Enter for none) | mode `jira`, `linear` or `github`, `--task-url` not given |

Every answer goes through the same validation as the flags. When any task flag (`--tasks`, `--task-pattern`, `--task-prefix`, `--task-url`) is given, question 7 is not asked and the rules of the flags apply unchanged; with `--tasks auto`, `--tasks jira` and so on only the follow-up questions 8-10 still apply. In a terminal the legacy default and its note are replaced by these questions.

After the last answer, `init` prints the equivalent command on standard error, so it can be repeated in a script: the flags that were given, and the answers that differ from the flag defaults. Values are quoted for a POSIX shell.

```
equivalent command: cairn init --name demo --repos server --tasks auto --task-prefix FEAT
```

Ctrl-D (end of input), Ctrl-C or a read error at any question cancels `init`: it prints `cairn: init canceled`, exits with code 1 and has written nothing (no `.cairn` folder, no registry entry, no database). All questions are asked before anything is written.

Without a terminal (a script, CI, an agent, piped standard input), or with `--no-input`, `init` never prompts and never reads standard input; it behaves exactly as described under Task numbering.

## Output

Project mode:

```
initialized project tidepool (project mode) at /work/tidepool
database: /work/tidepool/.cairn/cairn.db
```

Machine mode prints `(machine mode)` and the database path under the data directory, `<data directory>/projects/tidepool/cairn.db`.

## Exit codes

- `0`: project created and registered.
- `1`: runtime error: the questions were canceled (Ctrl-D or Ctrl-C), the name is already registered, the folder is already registered, the folder already holds a `.cairn` store, `--path` is not a folder, or a file cannot be written.
- `2`: usage error: a positional argument was given, `--project` was combined with a flag it forbids, `--repo` or `--local` was given without `--project`, `--name` or `--context` with `--project` but without `--local`, or `--task-pattern` was combined with `--tasks auto`, `jira`, `linear` or `github`.
- `3`: invalid input: name, mode, repos, context, task mode, prefix, URL template or task pattern fails validation, including `--tasks custom` without `--task-pattern` and `--task-prefix` with a mode other than `auto`. Every bad field is listed on standard error, task flags included.

## Examples

```
cd /work/tidepool && cairn init --name tidepool --repos server,client --tasks auto --context "Tide data service."
```

Creates `/work/tidepool/.cairn/cairn.db` with automatic task numbering (`T-1`, `T-2`, ...) and prints the two lines in Output. Exit code 0.

```
cairn init --name tidepool --repos server,client --mode machine --path /work/tidepool --tasks jira
```

Creates the database under the data directory and registers `tidepool` at `/work/tidepool`, on a machine where `tidepool` is not yet registered; tasks are Jira keys. Exit code 0.

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
cairn init --name tidepool --repos server --tasks custom --task-pattern '^(?P<base>TP-\d{2})$'
```

Creates a project in mode `custom` with the given pattern; same as leaving out `--tasks`. Exit code 0.

```
cairn init --name tidepool --repos server --tasks jira --task-pattern '^TP-\d+$'
```

Exit code 2: `--task-pattern` only goes with `--tasks custom`.

```
cd /work/tidepool-web && cairn init --project tidepool --repo web
```

Attaches the folder to `tidepool`: writes `.cairn/links.json`, records the child in the parent's database (commit that file), and prints where. Exit code 0.

```
cd /work/tidepool-web && cairn init --project tidepool --repo web --local --name web
```

Creates a database of its own for the folder, linked to `tidepool`. Exit code 0.

```
cairn init --project tidepool --repo web --tasks auto
```

Exit code 2: a child takes the parent's task settings.

```
cairn init --name Tidepool --repos server --tasks auto
```

Exit code 3: the name has an uppercase letter.

An interactive session (the answers after the colons are typed; empty ones are Enter):

```
$ cairn init --name demo
Where should the database live?
  1) project: the database lives in .cairn/ inside the project folder
  2) machine: the database lives in cairn's data directory, outside the folder
Your choice [project]:
Project root folder [/work/demo]:
Repos, comma-separated [demo]: server
Context, one line, optional: Tide data service.
How do you number your tasks?
  1) auto: cairn numbers your work for you (T-1, T-2, ...). Pick this if you have no ticket tracker, or you work alone.
  ...
Your choice [auto]:
Which tag do you want? Enter keeps T (T-1, T-2, ...), or type your own letters, for example FEAT (FEAT-1).
Tag [T]: FEAT
equivalent command: cairn init --name demo --repos server --context 'Tide data service.' --tasks auto --task-prefix FEAT
initialized project demo (project mode) at /work/demo
database: /work/demo/.cairn/cairn.db
```

```
cairn init --name demo --repos server --no-input
```

Never asks, even in a terminal; on purpose, no task flag is given, so it uses the legacy default numbering (mode `custom` with the default pattern) and prints its note. Exit code 0.

## Notes

- After a successful `init`, cairn asks GitHub for the latest release (3 seconds at most). If it differs from the installed version, one line goes to standard error: `cairn: a new version is available (1.2.0, installed: 1.0.0); install it with 'cairn update'`. Standard output, the exit code and the project are not affected, and any network problem is silent. Set `CAIRN_NO_UPDATE_CHECK` to any non-empty value to skip the check; development builds never check.
- `init` creates no notes. Start with `cairn add`.
- `--repos` is required. The names given here are the only values that `cairn add --repo` and `--impact` accept for this project.
- The legacy default task pattern accepts `TP-03`, `TP-03.1` and `ABCDE-1234`. Choose `--tasks` or pass `--task-pattern` when the task IDs differ.
- `.cairn/` in project mode lives inside the folder; `machine` mode keeps the folder clean.
- A project name or folder that is already registered is refused with exit code 1; `cairn projects` lists them.
