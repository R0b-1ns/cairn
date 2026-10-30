# cairn view

Writes a read-only website of the project's notes, to browse and filter them in a browser. It never changes the database.

## Usage

```
cairn view [--serve] [--port N] [--no-open] [--project NAME] [--json]
```

## Arguments

None. A positional argument gives exit code 2.

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--serve` | `false` | flag, no value | Also serve the site on `http://127.0.0.1:<port>/` until Ctrl-C. |
| `--port` | `0` | 0 to 65535 | With `--serve`: the port to listen on. `0` picks a free port. |
| `--no-open` | `false` | flag, no value | With `--serve`: do not open the browser. |
| `--project` | (empty) | a registered project name | Project to read. Default: found from the current directory, or env `CAIRN_PROJECT`. |
| `--json` | `false` | flag, no value | Print one JSON object instead of text. |

`--port` and `--no-open` without `--serve` give exit code 2.

## Output

The site is written to a `view` folder next to the database: `<root>/.cairn/view` for a project-mode project, `<CAIRN_HOME>/projects/<name>/view` for a machine-mode project. Each run rewrites it.

```
wrote /work/tidepool/.cairn/view
open /work/tidepool/.cairn/view/index.html
```

With `--serve`, a third line follows once the server is listening, and the browser opens on that URL unless `--no-open` is given:

```
serving http://127.0.0.1:53817/ (press Ctrl-C to stop)
```

The server listens on 127.0.0.1 only. If the browser cannot be opened, `cairn: warning: could not open the browser: ...` goes to standard error and the server keeps running.

With `--json`, one object: `{"dir": "...", "index": ".../index.html", "url": ""}`. `url` is empty without `--serve`; with `--serve` the object is printed once the server is listening.

## Exit codes

- `0`: written (and, with `--serve`, stopped with Ctrl-C).
- `1`: runtime error: the site cannot be written (for example the `view` folder is a symlink), or the port cannot be bound.
- `2`: usage error: a positional argument, `--port` outside 0 to 65535, `--port` or `--no-open` without `--serve`.
- `4`: no project found.

## Examples

```
cairn view --serve
```

Writes the site, serves it on a free port of 127.0.0.1 and opens the browser. Exit code 0 after Ctrl-C.
