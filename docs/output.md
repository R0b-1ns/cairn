# Output

What each command prints, in text and with `--json`. Results go to standard output; errors go to standard error (see `exit-codes.md`).

## Text layout

Columns are separated by two spaces unless noted. A type or status is padded to its width, so one extra space may follow it.

### Writes

`cairn add` prints one line, then one line per entry, with its ID and type:

```
note n-4k7q2m added (TP-03, done)
  e-7h2m9q change
  e-3d8k5r decision
  e-5w4n8p open
```

With no task, the header is `note n-4k7q2m added (done)`.

`cairn amend` prints:

```
addendum a-9x3b2c added to n-4k7q2m
  e-1q6v8m decision
```

### show

`cairn show <id>` prints the full item, with nothing clipped. Entries are listed in the order written, and show each full text.

For a note:

```
n-4k7q2m  Add tidal index
task: TP-03   repo: server   agent: builder   model: claude-opus-5-5   at: 2026-10-30T10:00:00Z
status: done
impacts: client
commits: 3f9c2ab
entries:
  e-7h2m9q  change   Added the hourly tidal index table.
  e-3d8k5r  decision [user] Index buckets are hourly, to match the sensor rate.
  e-5w4n8p  open     Should old buckets be pruned after 90 days?
none: note, helper
```

- Line 1: note ID, two spaces, full title.
- `task` shows `-` when empty. `status` shows `status: <current>`. If the current status differs from the note's own, it adds `(set when written: <own>)`.
- `impacts`, `commits`, `none` and `resolves`/`supersedes` lines appear only when they have values.
- Each entry: two spaces, ID, two spaces, type padded to 8, a space, `[user]` or `[agent]` for decisions, the text. A closed entry ends with `  (resolved by <id>)` or `  (superseded by <id>)`.
- Continuation lines of multi-line text are indented under the text: 6 spaces in `show` of a note, 4 in `show` of an addendum or an entry.
- Each addendum follows the note as a block: a line `addendum <a-id>  <at>  <agent> (<model>)`, optional `  status: <status>`, its entries, then its `resolves:` and `supersedes:` lines.

For an addendum, the first line is `<a-id>  addendum to note <n-id>`. For an entry, the first line is `<e-id>  <type>  in note <n-id>` (with ` via addendum <a-id>` when it came from an addendum), then the entry line.

### list

One line per entry, newest first. The example uses `cairn list --all`, which is the only way the resolved entry appears:

```
e-7h2m9q  change   TP-03  server  2026-10-30  Added the hourly tidal index table.  n-4k7q2m
e-3d8k5r  decision TP-03  server  2026-10-30  [user] Index buckets are hourly, to match the sensor rate.  n-4k7q2m
e-5w4n8p  open     TP-03  server  2026-10-30  Should old buckets be pruned after 90 days?  (resolved by a-9x3b2c)  n-4k7q2m
```

| Column | Meaning |
|---|---|
| 1 | Entry ID. |
| 2 | Entry type, padded to 8. |
| 3 | Task of the parent note, or `-`. |
| 4 | Repo of the parent note. |
| 5 | Date (`YYYY-MM-DD`) of the note or addendum that holds the entry. |
| 6 | Text: `[by] ` for decisions, then the text clipped to 160 characters. |
| 7 | Only with `--resolved` or `--all`: `(resolved by <id>)` or `(superseded by <id>)`. |
| last | Note ID that owns the entry. For an addendum entry, the parent note. |

With no matches: `no entries`. If more match than `--max` allows: `... 12 more (raise --max)` on its own line, unindented.

### brief

```
TP-03: 2 note(s)
notes:
  n-4k7q2m  TP-03    done        2026-10-30  Add tidal index (server, builder)
  n-8c2e6t  TP-03.1  in_progress 2026-10-31  Tidal index follow-up (client, builder)
changes:
  e-7h2m9q  Added the hourly tidal index table.  (n-4k7q2m)
decisions in force:
  e-3d8k5r  [user] Index buckets are hourly, to match the sensor rate.  (n-4k7q2m)
open points:
  e-5w4n8p  Should old buckets be pruned after 90 days?  (n-4k7q2m)
```

- The header is `<task>: <count> note(s)`. Notes are listed oldest first: ID, task, status (padded to 11), date, title, then `(repo, agent)`.
- Then three sections: `changes:`, `decisions in force:`, `open points:`. Entries are listed oldest first. Each shows ID, text, and the note (`(<n-id>)`, or `(<n-id> via <a-id>)`).
- An empty section prints `  none`.
- With no notes: `no notes for task <task>`.
- `--max` applies to each list separately. A cut list ends with `  ... N more (raise --max)`, indented by two spaces.

### recent

```
TP-03  2026-10-30  done        Add tidal index  (2 notes)
n-9p4w2h  2026-10-29  in_progress Something without a task  (1 note)
```

One line per subject, most recently active first: the task base or, for a note with no task, the note ID; then the date of its latest activity; status (padded to 11); title; the count of notes. With none: `no notes yet`. `recent` takes a count (default 10), not `--max`.

### search

```
n-4k7q2m  title    Add tidal index  (n-4k7q2m)
e-7h2m9q  change   Added the hourly tidal index table.  (n-4k7q2m)
```

Columns: ID, kind (`title` for a note title, otherwise the entry type; padded to 8), the matching snippet clipped to 160 characters, then the note, plus `via <a-id>` for addendum entries. Best match first. With none: `no matches`.

### check

```
done tasks without notes:
  TP-04  no note for this task
done tasks whose latest note is not done:
  TP-03  latest note n-4k7q2m is in_progress
open points older than 14 days:
  e-5w4n8p  2026-09-01  Should old buckets be pruned after 90 days?  (n-4k7q2m)
```

Each section appears only when it has findings. With none: `ok: nothing to report` (exit 0). Any finding exits 5.

### projects

```
NAME      MODE     NOTES  PATH
tidepool  project  12     /Users/example/Project/tidepool
```

Columns: name, mode, note count (`?` when the database cannot be read), path. A path that no longer exists ends with `  [missing]`. With none: `no projects registered`.

### context

```
project: tidepool (project mode)
repos: server, client
task pattern: ^(?P<base>[A-Z]{2,5}-\d{1,4})(?:\.(?P<ext>\d{1,3}))?$

Tide data service.
```

The blank line and context appear only when the project has a context.

### export (markdown)

`cairn export` with the default `--format md` writes a readable document, not an import file. It has a `# <project>` heading, the context, then for each note a `## <n-id>  <title>` heading, a metadata line `task: ... · repo: ... · agent: ... (<model>) · at: ... · status: ...`, one `### <type>` section per type, with each entry as a list item that starts with its ID in backticks, then `none:`, `impacts:`, `commits:` lines, and addenda as `#### addendum ...` blocks. Use `--format json` to make an import file (see `note-json.md`).

## --json

Every command that lists or shows data accepts `--json`, which prints indented JSON (two spaces) and nothing else. Text clipping does not apply: `text` and `title` are complete.

| Command | JSON shape |
|---|---|
| `add` | The stored note: `id`, `agent`, `repo`, `at`, `task`, `model`, `title`, `status`, `entries` (with IDs), and `none`, `impacts`, `commits`, `resolves`, `supersedes` when set. |
| `amend` | The stored addendum: `id`, `note_id`, `agent`, `model`, `at`, `status` when set, `entries`, `resolves`, `supersedes`. |
| `show <n-id>` | The note, plus `current_status` and `addenda` (an array, possibly empty). |
| `show <a-id>` | The addendum. |
| `show <e-id>` | One entry object (see below). |
| `list` | An array of entry objects. |
| `search` | An array of hits: `kind`, `id`, `note_id`, `addendum_id`, `type`, `snippet`. |
| `brief` | An object: `task`, `notes` (headers), `changes`, `decisions`, `open`. Not capped by `--max`. |
| `recent` | An array: `subject`, `last_at`, `status`, `title`, `note_id`, `notes`. |
| `check` | An object: `missing_notes`, `not_done`, `old_open`, `ok`. |
| `context` | The project object: `name`, `mode`, `context`, `repos`, `task_pattern`. |
| `projects` | An array: `name`, `mode`, `path`, `notes` (`-1` when unreadable), `missing`. |

An entry object always has every key: `id`, `type`, `text`, `by` (empty unless a decision), `note_id`, `addendum_id` (empty unless from an addendum), `task`, `repo`, `at`, `closed` (boolean), `closed_by`, `closed_at`.

The list-style commands (`list`, `search`, `brief` sections, `recent`, `check`, `projects`) print `[]` for an empty list, never `null`. Exceptions: `add` prints `entries` as `null` when the note has no entries, and `amend` omits `entries` when the addendum has none.

**JSON is capped by `--max` where text is.** `list`, `search` and `check` return at most `--max` items in JSON and print no "more" marker. Use `--max` with a larger value to get everything. `brief` JSON is not capped.

## --max

`--max N` caps printed items. Default 50, minimum 1 (see `limits.md`). It applies to `list`, `search`, `brief` (per section) and `check` (old open points only). The marker tells how many were left out:

```
... 12 more (raise --max)
```

## Identifiers

- IDs are printed in full, never shortened: `n-4k7q2m` (note), `a-9x3b2c` (addendum), `e-7h2m9q` (entry).
- Task IDs are printed exactly as stored.
- Dates are `YYYY-MM-DD`, from the UTC time stored. Full times appear only in `show`, JSON and the markdown export.

## Ordering

| Command | Order |
|---|---|
| `list` | Newest first, by the time of the note or addendum that holds the entry. |
| `brief` | Oldest first within each section; notes oldest first. |
| `recent` | Most recently active subject first. |
| `search` | Best match first. |
| `show` | Entries in the order written; addenda oldest first. |

## Clipping

- In list-style text (`list`, `brief`, `search`, `check`, `recent`), each text is flattened to one line (line breaks and tabs become spaces, runs of spaces collapse) and clipped to **160 characters**. A clipped text ends with `…`.
- `show` never clips. Multi-line text keeps its line breaks, indented as described above.
- JSON never clips.

## Terminal safety

Every write to standard output or standard error passes through one sanitizer, so stored text cannot control the reader's terminal:

- ANSI and other escape sequences are removed, with their parameters: control sequences (`ESC [ ...`), string sequences (`ESC ]`, `ESC P`, `ESC X`, `ESC ^`, `ESC _`, ended by BEL or `ESC \`), and two-character sequences.
- Control characters are removed, except newline and tab.
- U+0080 to U+009F, U+2028 and U+2029 are removed.
- JSON output is not changed in practice: the JSON encoder already writes control characters and U+2028/U+2029 as escapes.

Stored text cannot contain these characters to begin with (see `limits.md`), so the sanitizer is a second line of defense for data written by other tools.
