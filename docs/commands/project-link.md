# cairn project link

Records where a child of a project lives on this machine. A fresh clone of an attached child does not work until this is run, because cairn refuses a pointer file that the parent project does not know on this machine.

## Usage

```
cairn project link [--project NAME] <child> <path>
```

## Arguments

| Argument | Required | Meaning |
|---|---|---|
| `<child>` | yes | The name of an active child of the project, e.g. `web`. |
| `<path>` | yes | The folder of that child on this machine. |

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--project NAME` | the project found from the current directory (env `CAIRN_PROJECT`) | The parent project. It must be registered on this machine (`cairn project add`). Use it when you run the command from the child's folder or from anywhere else. |

## Output

```
linked child web of project tidepool at /work/web
```

The folder is checked first. A local child must hold a database that names this parent (name and project id) and carries the child's name. An attached child must hold a `.cairn/links.json` whose parent, repo and parent id match. Nothing is recorded when the check fails. The path is stored in the registry only; no database changes.

## Exit codes

- `0`: linked.
- `1`: runtime error: the project is unknown or not registered; the name is not an active child; the folder does not exist or is not that child; the registry cannot be written.
- `2`: usage error: not exactly two arguments, or the child name is not a valid name.

## Examples

```
cairn project link --project tidepool web /work/web
```

Records `/work/web` as the folder of the child `web`. Prints `linked child web of project tidepool at /work/web`. Exit code 0.

```
cairn project link web /work/other
```

Exit code 1 when `/work/other` is not the child `web` of the project found from the current directory.

## Notes

- The warning that cairn prints for an unknown child folder names this command, with `--project`, ready to run.
- A child whose database has an older schema is refused with a message to run any cairn command inside that repo once; cairn never upgrades another repo's database.
- A name that is not an active child is refused with the `cairn init --project` command that attaches a folder again.
- Check that you trust the folder before linking it: its notes are data written by others.
