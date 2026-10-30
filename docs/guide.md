# cairn guide

`cairn` stores immutable work notes in SQLite and returns them in small slices. Use it to learn what was done, decided and left open for a task, and to leave the same for the next reader. Nothing in a note can be edited or deleted; later facts go in addenda.

## The routine

1. Before working on a task, read its brief: `cairn brief TP-03`
2. Do the work.
3. At the end, write one note: `cairn add ...` (template below).
4. Later, to add a fact, close an open point or replace a decision, never edit: `cairn amend n-4k7q2m ...`

Every output carries IDs: `n-` note, `a-` addendum, `e-` entry. Pass them to `show`, `--resolves` and `--supersedes`.

## Read commands

| Command | Use |
|---|---|
| `cairn brief <task>` | One merged view of a task and its extensions: notes, changes, decisions in force, unresolved open points. Start here. |
| `cairn list [filters]` | Fine selection. `--type change\|decision\|open\|note\|helper`, `--task`, `--repo`, `--impacts`, `--since`, `--until`. Resolved open points and superseded decisions are hidden; `--resolved` shows only those with what closed them; `--all` shows everything. |
| `cairn recent [N]` | The last N subjects handled (default 10). |
| `cairn show <id>` | The full note, addendum or entry, nothing clipped. |
| `cairn search <terms>` | Full-text search; all terms must match. Put `--` before a term that starts with `-`. |
| `cairn check` | `--done TP-01,TP-02` or `--done-file FILE` lists done tasks that have no note or whose latest note is not done; also open points older than `--older-than` days (default 14). Exit code 5 when it finds something. |
| `cairn context` | Project name, repos, task pattern and context. |
| `cairn projects` | Every project registered on this machine. |

Common flags: `--json` for JSON, `--max N` to cap the number of items (default 50), `--project NAME` to pick a registered project.

## Write a note

```
cairn add --agent builder --repo server --task TP-03 --model claude-opus-5-5 \
  --title "Add tidal index" --status done \
  --change "Added the hourly tidal index table." \
  --decision "user:Index buckets are hourly, to match the sensor rate." \
  --open "Should old buckets be pruned after 90 days?" \
  --none note,helper --impact client --commit 3f9c2ab
```

Rules:

- Required: `--agent`, `--repo` (one of the project's repos), `--model`, `--title`, `--status` (`done`, `in_progress`, `blocked`). `--task` is optional and must match the project's task pattern.
- Five sections exist: `change` (short summary of what was done), `decision` (a choice and its reason), `open` (an open point), `note` (something worth knowing), `helper` (a reusable building block: path and purpose). Each needs at least one entry or must be listed in `--none`. Never write "none" as an entry.
- `--decision` text starts with `user:` or `agent:` to say who decided.
- `--impact` lists other repos affected. `--commit` attaches commit hashes.
- Never put secrets, tokens or personal data in a note; `cairn` warns when it sees a likely secret but still saves the note.
- A bad note is refused with exit code 3 and the list of fields to fix.
- To write a note from another process, send JSON on standard input: `cairn add --stdin`.

```
echo '{"agent":"builder","repo":"server","task":"TP-03","model":"claude-opus-5-5","title":"Add tidal index","status":"done","entries":[{"type":"change","text":"Added the hourly tidal index table."}],"none":["decision","open","note","helper"]}' | cairn add --stdin
```

## Add to a note

```
cairn amend n-4k7q2m --agent builder --model claude-opus-5-5 --status done \
  --resolves e-7h2m9q --note "Pruning was added in the nightly job." \
  --decision "agent:Keep 90 days." --supersedes e-3d8k5r
```

- `--resolves <entry id>` closes an open point; `--supersedes <entry id>` replaces a decision. Targets may be in any note. The old text stays readable.
- `--status` sets a new status. A note's current status is the one from its most recent addendum that carries a status.
- `cairn add` accepts `--resolves` and `--supersedes` too.
- An addendum needs at least one entry, a status, `--resolves` or `--supersedes`.

## Exit codes

0 ok, 1 runtime error, 2 usage error, 3 invalid input (fields listed), 4 no project found, 5 `check` found problems.

## Set up and move data

- `cairn init --name tidepool --repos server,client --context "Tide data service."` creates a project in the current folder (`--mode machine` keeps the database outside the folder). Only `init` creates a database.
- `cairn export --format json --out notes.json` and `cairn import notes.json` move notes between databases. Import validates every note and never changes existing data.
- `cairn project move|add|remove` maintains the machine registry.

Every command has `--help`. Full reference: the `docs/` folder of the repository.
