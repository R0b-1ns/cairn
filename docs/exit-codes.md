# Exit codes

Every cairn command exits with one of six codes. Test the code, not the message text.

| Code | Meaning | Occurs when | Examples |
|---|---|---|---|
| 0 | ok | The command did what was asked, including when there is nothing to show. | `cairn add` wrote a note. `cairn list` found no entries (prints `no entries`). `cairn check` found nothing (prints `ok: nothing to report`). `cairn --help`. |
| 1 | runtime error | The command could not do the work, or refused a store or input that is well-formed. | A database failed verification: `immutability trigger notes_no_update was altered: database refused`. A store owned by another user: `refusing ...: owned by another user`. `cairn show n-4k7q2m` for an ID that does not exist: `n-4k7q2m not found`. `cairn init` where a store already exists. `cairn amend n-0000000` for a missing note: `note n-0000000 not found`. `cairn export --out FILE` when FILE exists without `--force`. `cairn import` refused because an ID exists with different content: `import refused, nothing was imported`. A registry file that is corrupt. |
| 2 | usage error | The command line is wrong: unknown command, flag, or argument count, or a value that is not allowed by a flag. | `cairn frobnicate`: unknown command. `cairn add --bogus`: unknown flag. `cairn show` with no ID: `usage: cairn show <id>`. `cairn list --type note,open`: `--type must be one of ...`. `cairn list --max 0`: `--max must be 1 or more`. `cairn add --stdin --title x`: `--stdin cannot be combined with note flags`. `cairn list --task tp-03`: the task does not match the pattern. `cairn search` with no terms. |
| 3 | invalid input | Input was read but refused: a field fails validation, JSON is malformed or has unknown keys, input is larger than its limit, or an import file is not a cairn export. | `cairn add --stdin` with `"repo": "web"`. `cairn add --stdin` with an unknown key. `cairn amend n-4k7q2m --stdin` whose JSON has `note_id`. `cairn import` on a file whose `format` is not `cairn-export`. `cairn init` with a bad `--name` or `--mode`. A `resolves` target that is not an open entry: `resolves: e-7h2m9q: no such entry`. Input over 1 MiB. |
| 4 | no project | No project could be found or selected. | `cairn list` run outside any project folder: `no cairn project found from ...; run 'cairn init' ...`. `cairn list --project NAME` where NAME is not registered. `CAIRN_PROJECT` naming a project whose `.cairn/cairn.db` was removed. |
| 5 | check found problems | `cairn check` reported at least one finding. Output is still printed. | A done task with no note. A done task whose latest note is not `done`. An open point older than `--older-than` days. |

Code 5 is produced only by `check`. Every other command uses 0 to 4.

## Standard error and standard output

- Results go to standard output. Errors and warnings go to standard error. Nothing else is printed.
- Errors: `cairn: <message>`, one line.
- Usage errors from a command's flags: `cairn <command>: <flag error>`, for example `cairn add: flag provided but not defined: -bogus`.
- Unknown command: `cairn: unknown command "frobnicate" (run 'cairn --help')`.
- Validation: the first line names how many fields to fix. Each following line is two spaces, the field, `: `, and the message.

```
cairn: invalid input, 2 field(s) to fix:
  repo: "web" is not an allowed repo (allowed: server, client)
  none: section note has no entry: add one or list it in none
```

- Secret warnings: `cairn: warning: possible secret (<rule>) in <field>; ...`. A warning does not change the exit code.
- Every byte written to standard output or standard error passes through the sanitizer described in `output.md`.
