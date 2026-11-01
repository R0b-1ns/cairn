# cairn project unlink

Detaches a child from its parent project. It never deletes a database or a note.

## Usage

```
cairn project unlink [--project NAME] [<child>]
```

## Arguments

| Argument | Required | Meaning |
|---|---|---|
| `<child>` | no | The name of an active child, to detach it from the parent's side. Without it the command runs from inside a child folder and detaches that child. |

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--project NAME` | the project found from the current directory (env `CAIRN_PROJECT`) | The parent project, for the form with `<child>`. |

## Output

```
unlinked child web from project tidepool (repo web stays in the project; no database or note was deleted)
```

From inside a child:

```
unlinked web from project tidepool (no database or note was deleted)
```

What happens:

- The parent records that the child is detached and forgets its path in the registry. The repo name stays among the parent's repos, so existing notes keep a valid repo.
- A local child becomes a standalone project: its database records that it has no parent.
- An attached child loses its `.cairn/links.json`.
- With `<child>`, the child's own side is changed only when its folder is known on this machine; otherwise a `cairn: warning` on standard error says to run `cairn project unlink` in that folder.
- Without `<child>`, when the parent is not registered or reachable on this machine, only the child's own link is cleared and a `cairn: warning` names the command to finish on the parent: `cairn project unlink --project <parent> <child>`.

## Exit codes

- `0`: unlinked (warnings do not change it).
- `1`: runtime error: the name is not an active child; the folder is not a child; a database cannot be written.
- `2`: usage error: more than one argument.
- `4`: no project found from the current directory (form without `<child>`).

## Examples

```
cairn project unlink web
```

Run in the parent: detaches `web`. Exit code 0.

```
cd /work/web && cairn project unlink
```

Detaches the child from inside its own folder, also for an attached child that this machine could not resolve.

## Notes

- Link again later with `cairn init --project <parent> --repo <repo>` in the folder of an attached child; the parent's repo list and all notes are unchanged.
- A local child that was unlinked keeps its database as a standalone project; `cairn init --project` refuses a folder that already holds a database, so it is not re-attached that way.
