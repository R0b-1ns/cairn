# cairn add

Creates one immutable work note, from flags or from one JSON object on standard input.

## Usage

```
cairn add --agent NAME [--repo REPO] --model MODEL --title TITLE --status STATUS [--task TASK] [--new-task] [--ref URL] [--author "Name <email>"] [--branch NAME] [--change TEXT]... [--decision TEXT]... [--open TEXT]... [--note TEXT]... [--helper TEXT]... [--none TYPES] [--impact REPO]... [--commit HASH]... [--resolves ID]... [--supersedes ID]... [--project NAME] [--json]
cairn add --stdin [--project NAME] [--json]
```

## Arguments

None. A positional argument gives exit code 2.

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--agent` | (empty) | any name, 1-80 characters | Agent name. Required in flag mode. |
| `--repo` | (empty) | a repo listed in the project's repos | Repo the work was done in. Optional when the project has a default repo (`cairn context` shows it); otherwise required. With `--stdin`, a missing `repo` uses the default the same way. |
| `--task` | (empty) | a task ID matching the project task pattern, e.g. `TP-03`, `TP-03.1` | Task ID. Optional. |
| `--new-task` | `false` | flag, no value | Assigns the next task number of the project (`T-1`, `T-2`, ...). Only in a project in `auto` mode, and not together with `--task`. |
| `--ref` | (empty) | URL starting with `http://` or `https://`, at most 500 characters | Link to the ticket this work belongs to. Optional. |
| `--author` | (empty) | `Name <email>`; name at most 80 characters, email at most 120, one line | Author of the note. Default: env `CAIRN_AUTHOR`, then git `user.name` and `user.email` of the current directory. A value read from env is refused when invalid (exit 3); an invalid value read from git is dropped with a warning on standard error. Without `<...>` the whole text is the name. |
| `--branch` | (empty) | one line, at most 200 characters | Branch the work was done on. Default: the current git branch of the current directory; empty on a detached HEAD or outside a repository. An invalid git value is dropped with a warning; an invalid `--branch` is refused (exit 3). |
| `--model` | (empty) | any name, 1-80 characters | Model that did the work. Required in flag mode. |
| `--title` | (empty) | one line, 1-200 characters | Short title. Required in flag mode. |
| `--status` | (empty) | `done`, `in_progress`, `blocked` | Note status. Required in flag mode. |
| `--change` | (empty) | text | Summary of what was done. Repeatable. |
| `--decision` | (empty) | text starting with `user:` or `agent:` | A choice and its reason; the prefix names who decided. Repeatable. |
| `--open` | (empty) | text | An open point or unresolved question. Repeatable. |
| `--note` | (empty) | text | Something worth knowing: a pitfall or a fact. Repeatable. |
| `--helper` | (empty) | text: path and purpose | A reusable building block. Repeatable. |
| `--none` | (empty) | comma-separated `change`, `decision`, `open`, `note`, `helper` | Sections declared empty. |
| `--impact` | (empty) | repo names of the project; repeatable or comma-separated | Other repos affected by the work. |
| `--commit` | (empty) | lowercase hex, 7-64 characters; repeatable or comma-separated | Commit hashes attached to the note. |
| `--resolves` | (empty) | ID of an open entry, `e-...` | Closes that open point. Repeatable. |
| `--supersedes` | (empty) | ID of a decision entry, `e-...` | Replaces that decision. Repeatable. |
| `--stdin` | `false` | flag, no value | Read the note as one JSON object from standard input. Only `--project` and `--json` may be combined with it. |
| `--project` | (empty) | a registered project name | Project to write to. Default: found from the current directory, or env `CAIRN_PROJECT`. |
| `--json` | `false` | flag, no value | Print the saved note as JSON. |

With `--stdin` the JSON object uses the keys `agent`, `repo`, `task`, `model`, `title`, `status`, `entries` (objects with `type`, `text` and, for decisions, `by`), `none`, `impacts`, `commits`, `resolves`, `supersedes`, and optionally `new_task`, `ref`, `author_name`, `author_email`, `branch`. Unknown keys are refused. `id`, `at` and entry `id` must be absent: cairn assigns them.

## Output

Text, on success:

```
note n-4k7q2m added (TP-03, done)
  e-3w9k1d change
  e-7h2m9q decision
  e-5c8r0t open
```

Each line after the first is one entry: its ID and its type. A note without a task prints `(done)` in place of `(TP-03, done)`.

Invalid input prints on standard error, and nothing is saved:

```
cairn: invalid input, 2 field(s) to fix:
  repo: "docs" is not an allowed repo (allowed: server, client)
  entries[0].by: a decision needs by "user" or "agent"
```

A likely secret in a text field prints a warning on standard error (`warning: possible secret (...) in <field>; ...`). The note is still saved.

`--json` prints the saved note: `id`, `agent`, `repo`, `at` (RFC3339 UTC), `task` (omitted when empty; the assigned ID with `--new-task`), `author_name`, `author_email`, `branch`, `ref` (each omitted when empty), `model`, `title`, `status`, `entries` (each with `id`, `type`, `text`, and `by` for decisions), and `none`, `impacts`, `commits`, `resolves`, `supersedes` when set.

## Exit codes

- `0`: note saved.
- `1`: runtime error, for example the database cannot be opened or written.
- `2`: usage error: a positional argument, `--stdin` combined with a note flag, `--new-task` together with `--task`, or `--new-task` in a project whose task mode is not `auto` (the message names the mode; a project from before task modes is `custom`).
- `3`: invalid input; every bad field is listed on standard error. Includes an invalid `--author`, `--branch` or `--ref`, and `new_task` with `task` in JSON. Also an `--resolves` or `--supersedes` ID that does not exist.
- `4`: no project found.

## Examples

```
cairn add --agent builder --repo server --task TP-03 --model claude-opus-5-5 --title "Add tidal index" --status done --change "Added the hourly tidal index table." --none decision,open,note,helper
```

Prints `note n-4k7q2m added (TP-03, done)` and one `e-...` line for the change. Exit code 0.

```
cairn add --agent builder --repo server --task TP-03 --model claude-opus-5-5 --title "Add tidal index" --status in_progress --change "Tidal index built for the server repo." --decision "user:Index buckets are hourly, to match the sensor rate." --open "Should old buckets be pruned after 90 days?" --none note,helper
```

Prints the note ID and three entry lines (change, decision, open). The `user:` prefix is stored as `by: user`.

```
echo '{"agent":"builder","repo":"server","task":"TP-03","model":"claude-opus-5-5","title":"Add tidal index","status":"done","entries":[{"type":"change","text":"Added the hourly tidal index table."}],"none":["decision","open","note","helper"]}' | cairn add --stdin --json
```

Prints the saved note as JSON with a generated `id`. Exit code 0.

```
cairn add --agent builder --repo server --model claude-opus-5-5 --title "Pick buckets" --status done --decision "Keep hourly buckets." --none change,open,note,helper
```

Refused with exit code 3: the decision has no `user:` or `agent:` prefix.

```
cairn add --agent builder --repo server --new-task --model claude-opus-5-5 --title "Add tidal index" --status in_progress --change "Started the index." --none decision,open,note,helper
```

In an `auto` project, prints `note n-4k7q2m added (T-7, in_progress)` when `T-6` was the highest number so far. Exit code 0.

```
cairn add --agent builder --repo server --task TP-03 --ref https://example.org/browse/TP-03 --author "Ada Lovelace <ada@x.org>" --branch feat/tidal --model claude-opus-5-5 --title "Add tidal index" --status done --change "Added the index." --none decision,open,note,helper
```

Stores the ticket link, the author and the branch as given. Exit code 0.

## Notes

- Every section needs at least one entry or must be listed in `--none`. A section both listed in `--none` and given entries is refused.
- Never write the text `none` as an entry. Use `--none` instead.
- `--decision` text must start with `user:` or `agent:`.
- With `--stdin`, `author_name`, `author_email` and `branch` that are missing are filled the same way as the flags (env `CAIRN_AUTHOR`, then git).
- `new_task: true` with a `task` is refused with exit code 3; `new_task: true` in a project that is not in `auto` mode is exit code 2, as the flag.
- `--stdin` cannot be combined with any note flag other than `--project` and `--json`; that is exit code 2.
- Notes are immutable. To change, close or replace something, use `cairn amend` with the note ID.
- Limits: 100 entries per note, 4000 characters per entry text, 50 items per list flag.
- `--resolves` and `--supersedes` accept IDs from any note in the project.
- A note cannot be deleted or edited with cairn.
