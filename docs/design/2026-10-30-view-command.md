# `cairn view`: a browsable site for the human reader

## Intent

Cairn's CLI is built for agents. The human who owns the project also wants to read the notes: scan them, filter them, open one. `cairn view` generates a small static website from a project's notes, with a table of notes and one page per note. It is for the human only; agents keep using the CLI.

Why now: notes pile up, and reading them through `list`, `show` and `export` is slow for a person.

## Goals

- `cairn view` writes a static site, read-only, rebuilt from the database on every run.
- A central page lists notes in a table with filters. A left sidebar switches between four views: Notes (default), Decisions, Open points, Helpers. The three entry views list entries, not notes.
- One page per note, with an "agent card" at the top, then one titled section per part.
- Each note page has a "Download as Markdown" button, to share a note with a colleague. The file is generated with the site (one `.md` per note, next to its page) using the same Markdown rendering as `cairn export --format md` (`renderMarkdown` in `internal/cli/export.go`), so the downloaded text is identical to the CLI's. Works from disk and from `--serve`.
- Each view (Notes, Decisions, Open points, Helpers) has a "Download this view as Markdown" button. It downloads only what the current filters leave visible: on Notes, the Markdown of the filtered notes joined in table order; on the entry views, the filtered entries as a Markdown list with their note IDs. The file is built in the browser from data already in the page, so it needs no server.
- The whole site is in English: labels, filters, buttons, banner, column names, and messages. (This design document's mockups were shown in French only for the conversation.)
- `cairn view --serve` also starts a small local server on the generated site and opens the browser. Serving is optional; without the flag the command only writes files.
- The generated files are never committed and never indexed by agents.

## Non-goals

- No editing. The site writes nothing to the database. Corrections stay addenda made through the CLI.
- No live refresh. New notes appear on the next `cairn view`.
- No accounts, no remote access, no hosted version.
- No external assets (CDN, fonts, frameworks). The site must work offline.

## Anti-goals

- Breaking or changing any existing database. A failure here would be worse than not shipping the feature.
- Making the site a second source of truth that an agent might read and trust over the CLI.

## Constraints

- **Existing databases stay valid.** The command is strictly read-only on the database. It adds no table, column, index, trigger or `PRAGMA user_version` change, because every open verifies the schema exactly (see `docs/storage.md`, "Verification on every open"). No store change is needed: the command uses the data already readable through the store.
- Written in Go, with no new dependency. Page templates are embedded in the binary.
- Filtering runs in plain JavaScript in the page. Data is embedded in the page or in a local file next to it, so the site also works opened straight from disk.
- The command follows the existing conventions: project lookup, `--project`, `--json` where relevant, exit codes, `docs/commands/<name>.md`, and the docs tests in `internal/cli/docs_test.go`.

## Decisions

Decisions below were made by the owner unless marked *builder's call* or *my call*.

- **Table rows are notes.** Columns: date, task, title, repo, agent, current status, number of open points not resolved. Filters: status, task, repo, agent, date range, text search.
- **Layout B**: sidebar on the left holds the view links and the filters; the table is on the right.
- **Note page**: an agent card at the top (agent, model, repo, impacts, task, date, current status, and "written status" when it differs), then one titled section per entry type in the order of the data model: Changes, Decisions, Open points, Notes, Helpers, then Addenda. Empty sections are shown greyed as "none", since every note declares all five types. A superseded decision or a resolved open point is struck through and links to what closed it.
- **`--serve` is an option**, not the default.
- *My call:* output goes to `.cairn/view/` in project mode, and to a `view/` folder under the project's data folder in machine mode. The command writes a `.gitignore` containing `*` inside that folder, so nothing is committed and the project's own `.gitignore` is not touched.
- *My call:* the server listens on `127.0.0.1` only, on a free port, serves only the generated folder, and stops on interrupt.
- *My call:* the site is regenerated on every run of `cairn view`.

- **Data is already readable.** The store already exposes everything the pages need, with no change: `NoteHeaders`, `GetNote`, `Addenda`, `CurrentStatus`, `ListEntries` (which carries `closed`, `closed_by`, `closed_at`), and `Entry`. If the builder finds something missing anyway, the rule is to cut that detail from the site, not to touch the store or the schema.
- **Current status** follows `docs/data-model.md`: the status of the note's most recent addendum that carries one, else the note's own status. Values are `done`, `in_progress`, `blocked`. The status filter offers exactly these three. "Written status" is shown only when it differs.
- **One project per run**, the one found like any other command (`--project`, `CAIRN_PROJECT`, or the folder walk). No project column.
- *My call:* the Decisions, Open points and Helpers views are tables of entries: ID, task, repo, date, text, author (decisions only), state, and a link to the parent note page. They reuse the sidebar filters that make sense for entries (task, repo, agent, date range, text). Closed entries (superseded decisions, resolved open points) are hidden by default, with a "show closed" toggle, matching `cairn list`.
- *My call:* every page carries a visible banner saying it is generated from the database, read-only, and not authoritative; the CLI is the source of truth. `.gitignore` is the only mechanism against indexing; no agent-specific ignore file.
- *My call:* `cairn view --json` prints one object with the output folder, the entry file, and the URL when serving. Exit codes follow `docs/exit-codes.md`.

## Left to the builder

- Exact file and folder layout of the generated site, and URL scheme for note pages.
- Whether the data is embedded in each page or shared in one data file.
- Visual styling, within "clean and readable", including light and dark theme.
- Flag names beyond `--serve` (for example port and no-open), and the exact `.gitignore` handling in machine mode.
- How to open the browser on each platform.
- Test strategy, including a test that a database is byte-identical before and after `cairn view`.
