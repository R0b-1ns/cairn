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

- Link again later: for an attached child run `cairn init --project <parent> --repo <repo>` in its folder; for a local child run `cairn init --project <parent> --repo <repo> --local` (add `--name` when the project name differs from the repo). `init` adopts the existing standalone database when its project name and repo match and it has no parent; no note is changed.
- Without `<child>`, the parent is detached only when the id recorded in the child (pointer `parent_id` or the database's parent event) is present and equals the registered parent's id, and the parent has recorded this very folder for the child (`cairn project link`). Otherwise only the child's own link is cleared and a warning names the command to run on the parent.
- A child whose pointer was left behind refuses commands (exit code 4) until you run `cairn project unlink` inside it or `cairn project link ...`.
- Unlinking never migrates a database it only reads: a child on an older schema is left alone until a cairn command runs inside that repo.
