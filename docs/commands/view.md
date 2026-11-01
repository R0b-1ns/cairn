# cairn view

Writes a read-only website of the project's notes, to browse and filter them in a browser. When the project has local children (see `project.md`), their notes are part of the site. It never changes the database, apart from the one-time schema 3 to 4 migration that every command performs when it opens an older database (see `storage.md`).

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

The Notes table has Branch and Author columns next to Agent. Each cell shows the note's own branch or author; when the note's addenda were written on other branches or by other authors, those follow after a `+`. A column with no value shows `-`. Click a column header to sort by it: the first click sorts ascending, the second descending, and an arrow in the header shows the order. The Branch and Author columns sort like the others.

Every view filters by task, repo, agent, branch, author, a date range and text; Notes also filter by status. The branch and author filters follow `cairn list --branch` and `--author`: on the Notes view a note matches when the note or any of its addenda has that branch, or an author with that name or email; on the Decisions, Open points and Helpers views an entry matches with the branch and author of the note or addendum that holds it. The author filter lists each author by name, or by email when there is no name. These two filters offer only values that exist, so a project whose notes have no branch or author offers none. The text filter also searches author, branch and refs. Each view offers only the values of the rows it lists. The task filter follows the same rule as `cairn list --task`: a base task such as `TP-03`, shown as "TP-03 (incl. extensions)" when the view has some, selects the base and its extensions; an extension such as `TP-03.1` selects only itself. Decisions and open points that are closed are hidden until "Show closed" is ticked; when all of them are closed, the page says so.

Each note has its own page with a card: agent, model, repo, impacts, author (name and email), branch, refs, task, date, status and commits. A ref is a link only when it starts with `http://` or `https://`, and opens with `rel="noopener noreferrer"`. The refs are those stored for the note's task (on any of its notes or addenda); when there is none and the project has a task URL, the card shows the link built from it. Each addendum shows its own author, branch and ref.

Each note has a Markdown download, the same text as `cairn export --format md`, so it includes the origin line and refs. Each view has a "Download this view" button that exports only what is currently filtered, including by branch and author. A downloaded list of entries starts with `# <project> — <view>` and says `No matching <entries>.` when nothing is shown. In a downloaded list of decisions or open points, a closed entry ends with `(superseded by <id>)` or `(resolved by <id>)`, as in `cairn list`.

The pages work when opened straight from disk (`file://`) with no server, so `--serve` is optional.

## Local children

A parent opens each of its active local children read-only and merges their notes, newest first, with its own. The direction is parent to child only: a child's `view` shows only its own notes, and `view` never writes to a child's database. Attached children need nothing: their notes already are in the parent's database.

- The Notes table and the Decisions, Open points and Helpers tables get a Child column, and every view gets a Child filter, only when at least one note comes from a child. A project without local children produces the same site as before. The current project's own rows show `-` in the Child column. The Child column sorts like the others, the text filter also searches the child name, and the filter offers only the children that have rows in the view.
- A note page shows its child on the card. A note downloaded from a child starts with `# child: <name>`, and a downloaded list of entries marks an entry of a child with `child <name>` inside the parentheses after the note.
- A child that cannot be read (its folder is not known on this machine, it has no database, it holds a different project, or its schema is older than this build) is skipped and one line goes to standard error, naming the command to run, for example `cairn: warning: local child web of tidepool was not read: ...; to point to it, run: cairn project link --project tidepool web <path>`. The site is still written and the exit code is unchanged.
- Page files are named after note IDs. When two sources hold the same note ID (for example a copied database), `view` exits 1 with `cairn: cannot write the site: note <id> is in both project <parent> and child <name>` and writes no file.
- Child names and every other stored value are escaped in the HTML and in the data embedded in the pages, and cleaned of terminal control characters, like all other text.
- In the task filter, a child's tasks are grouped with the parent's task pattern.

## Exit codes

- `0`: written (and, with `--serve`, stopped with Ctrl-C or SIGTERM).
- `1`: runtime error: the site cannot be written (for example the `view` folder is a symlink), the port cannot be bound, or two sources hold the same note ID.
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
- The command never writes to the database. Opening a database still at schema 3 migrates it to schema 4 first, after a backup copy, as every command does.
