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

Ctrl-C and SIGTERM both stop the server; the command then exits 0. The `wrote` and `open` lines are printed before the server starts, so if the port cannot be bound they are printed and the command then exits 1.

## The site

The site is in English. It has four views, each a page with its own filters:

- **Notes**: one row per note.
- **Decisions**: every decision entry.
- **Open points**: every open-point entry.
- **Helpers**: every helper entry.

The filters are status, task, repo, agent, a date range and text. Each view offers only the values of the rows it lists. The task filter follows the same rule as `cairn list --task`: a base task such as `TP-03`, shown as "TP-03 (incl. extensions)" when the view has some, selects the base and its extensions; an extension such as `TP-03.1` selects only itself. Decisions and open points that are closed are hidden until "Show closed" is ticked; when all of them are closed, the page says so.

Each note has a Markdown download. Each view has a "Download this view" button that exports only what is currently filtered. In a downloaded list of decisions or open points, a closed entry ends with `(superseded by <id>)` or `(resolved by <id>)`, as in `cairn list`.

The pages work when opened straight from disk (`file://`) with no server, so `--serve` is optional.

## Exit codes

- `0`: written (and, with `--serve`, stopped with Ctrl-C or SIGTERM).
- `1`: runtime error: the site cannot be written (for example the `view` folder is a symlink), or the port cannot be bound.
- `2`: usage error: a positional argument, `--port` outside 0 to 65535, `--port` or `--no-open` without `--serve`.
- `4`: no project found.

## Examples

```
cairn view --serve
```

Writes the site, serves it on a free port of 127.0.0.1 and opens the browser. Exit code 0 after Ctrl-C.

```
cairn view --serve --port 8080 --no-open
```

Writes the site and serves it on `http://127.0.0.1:8080/` without opening the browser. Exit code 1 if port 8080 is in use, after the `wrote` and `open` lines.

```
cairn view --json
```

Writes the site and prints `{"dir": "...", "index": "...", "url": ""}`. Exit code 0.

## Notes

- Each run rewrites the `view` folder. Do not keep your own files in it.
- The folder is generated: it holds a `.gitignore` containing `*`, so it is never committed. It holds no source data and is safe to delete. See `storage.md`.
- The site is read-only and a copy: the database stays the source of truth. Run `cairn view` again to refresh it.
- Note text is escaped in the HTML and never inserted as HTML in the browser. See `security.md`.
- The command never writes to the database.
