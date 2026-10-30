# cairn context

Prints the project's name, storage mode, repos, task pattern and context text.

## Usage

```
cairn context [--project NAME] [--json]
```

## Arguments

None. A positional argument gives exit code 2.

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--project` | (empty) | a registered project name | Project to read. Default: found from the current directory, or env `CAIRN_PROJECT`. |
| `--json` | `false` | flag, no value | Print the project configuration as JSON. |

## Output

Text:

```
project: tidepool (project mode)
repos: server, client
task pattern: ^(?P<base>[A-Z]{2,5}-\d{1,4})(?:\.(?P<ext>\d{1,3}))?$

Tide data service. Sensor ingest, the tidal index and the browser client.
```

The blank line and context text are printed only when a context is set. `mode` is `project` or `machine`.

`--json` prints:

```json
{
  "name": "tidepool",
  "mode": "project",
  "context": "Tide data service. Sensor ingest, the tidal index and the browser client.",
  "repos": ["server", "client"],
  "task_pattern": "^(?P<base>[A-Z]{2,5}-\\d{1,4})(?:\\.(?P<ext>\\d{1,3}))?$"
}
```

## Exit codes

- `0`: printed.
- `1`: runtime error, for example the registry or database cannot be read.
- `2`: usage error: a positional argument was given.
- `4`: no project found (no `.cairn/cairn.db` above the current directory, and no matching `--project` or `CAIRN_PROJECT`).

## Examples

```
cairn context
```

Prints the project block shown in Output for the project of the current directory. Exit code 0.

```
cairn context --project tidepool --json
```

Prints the JSON object for `tidepool`, from any directory. Exit code 0.

```
cd /work && cairn context
```

Exit code 4 when `/work` is not inside a project; the message says to run `cairn init` or pass `--project NAME`.

## Notes

- Read `context` once per session to learn the allowed `--repo` values and the task ID format before writing a note.
- The task pattern is the regular expression that every `--task` value must match.
- Use `--project` or the `CAIRN_PROJECT` environment variable to address a registered project outside its folder.
