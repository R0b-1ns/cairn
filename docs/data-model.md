# Data model

Rules that every note, addendum, entry and link follows. Read this before writing or reading notes.

## Objects

| Object | What it is | Storage table | ID prefix |
|---|---|---|---|
| project | The one configuration row: name, mode, context, repos, task pattern. Written once by `init`. | `project` (row id 1) | none |
| note | One immutable work note: agent, repo, model, title, status, sections. | `notes` | `n-` |
| addendum | One immutable addition to one note: may set a status, add entries, resolve open entries, supersede decisions. | `addenda` | `a-` |
| entry | One typed piece of text. Belongs to exactly one note or exactly one addendum. | `entries` | `e-` |
| link | A `resolves` or `supersedes` record from a note or addendum to one entry. | `links` | none |

## Entry types

Exactly five types exist. Display order is the order below.

| Type | Meaning |
|---|---|
| `change` | Short summary of what was done. |
| `decision` | A choice and its reason. Has `by`: `user` or `agent`. |
| `open` | An unresolved point. Can be closed by a `resolves` link. |
| `note` | Something worth knowing: a pitfall or a fact. |
| `helper` | A reusable building block: path and purpose. |

## Sections rule (notes only)

Every note must cover all five types. For each type, either:

- the note has at least one entry of that type, or
- the type is listed in the note's `none` array.

A type listed in `none` must have no entries. An addendum has no such rule.

## Statuses

A status is one of `done`, `in_progress`, `blocked`. No other value is accepted, on notes or addenda.

## Immutability

- Ten triggers exist: `BEFORE UPDATE` and `BEFORE DELETE` on each of `project`, `notes`, `entries`, `addenda`, `links`. Names are `<table>_no_update` and `<table>_no_delete`. Each one aborts with `immutable: <table> rows cannot be modified or deleted`.
- No command updates or deletes a row. A correction is always an addendum.
- The `search` full-text index is not covered by the triggers. Code keeps it in step with inserts.

## IDs

Format: `<kind>-<6 characters>`.

| Kind | Prefix | Used for |
|---|---|---|
| note | `n-` | notes |
| addendum | `a-` | addenda |
| entry | `e-` | entries |

- The 6 characters come from `0123456789abcdefghjkmnpqrstvwxyz`: lowercase Crockford base32. The letters `i`, `l`, `o`, `u` never occur.
- Pattern for an entry ID: `^e-[0-9a-hjkmnp-tv-z]{6}$`. Same shape for `n-` and `a-`.
- Examples: `n-4k7q2m`, `a-9x3b2c`, `e-7h2m9q`.
- cairn assigns IDs randomly. A new ID is checked against every note, addendum and entry ID.
- Only `import` keeps IDs it is given. A given ID that already exists is refused.

## Timestamps

- `at` on a note or addendum is set by cairn at write time, in UTC, to whole seconds: `2026-10-30T10:00:00Z`.
- An entry has no time of its own. It takes the `at` of the note or addendum that holds it.
- Import accepts any RFC3339 timestamp and stores it in UTC, without fractional seconds.

## Tasks

- A note's `task` is optional. When set, it must match the project's task pattern (see `configuration.md`).
- The pattern's `base` group is stored as `task_base`. The `ext` group, when present, is stored as `task_ext`.
- A base task selects every note whose `task_base` equals it. `TP-03` selects notes for `TP-03` and for `TP-03.1`.
- An extension selects exactly that task. `TP-03.1` selects only notes for `TP-03.1`.
- Commands that take a task: `brief`, `list --task`, `check --done`, `export --task`. Each one follows the base and extension rule.
- `check` counts extension notes toward their base. Its "latest note" is the most recent note of the base, by the note's own `at`.
- A note with no task is its own subject in `recent`.

## Links: resolves and supersedes

- `resolves` targets must be `open` entries. `supersedes` targets must be `decision` entries. Any other type is refused with `is a <type> entry, expected <type>`.
- A target can be closed once per kind. A second `resolves` of the same open entry is refused with `already resolved`. A second `supersedes` of the same decision is refused with `already superseded`.
- A link is a separate record. The target entry's text and type never change.
- Closed state is computed at read time. An entry is closed when a link points at it:
  - an `open` entry closed by a link is **resolved**;
  - a `decision` entry closed by a link is **superseded**;
  - `closed_by` is the ID of the note or addendum that holds the link.
- A link can target an entry in any note, including an older one.
- A note or addendum can carry up to 50 `resolves` and up to 50 `supersedes`.

Effect on reads:

| Command | Default behavior |
|---|---|
| `list` | Hides resolved open points and superseded decisions. `--resolved` shows only them, with what closed them. `--all` shows everything. |
| `brief` | Shows every change. Shows decisions in force and open points that are not closed. |

## Current status

- A note's **current status** is the status of its most recent addendum that carries a status. "Most recent" means the latest addendum `at`; ties go to the later insertion.
- If no addendum carries a status, the current status is the note's own `status`.
- The stored note `status` never changes. `show` prints `status: <current> (set when written: <own>)` when the two differ.
- `check` and `recent` use the current status.

## none

- `none` is the list of entry types a writer declares empty. It is stored with the note and printed by `show` as `none: ...`.
- An entry whose text is `none` is refused. Declare empty sections in `none` instead.

## Example

A note in task `TP-03` with its own decision, then an addendum that resolves one open point and supersedes an earlier decision. The layout is simplified; it is not the exact output of any command (see `output.md`):

```
n-4k7q2m  TP-03  status: done
  e-5w4n8p  open      Should old buckets be pruned after 90 days?
  e-3d8k5r  decision  [user] Index buckets are hourly.

a-9x3b2c  status: done
  e-1q6v8m  decision  [agent] Keep 90 days.
  resolves: e-5w4n8p
  supersedes: e-3d8k5r
```

After that addendum: `e-5w4n8p` is resolved by `a-9x3b2c`, `e-3d8k5r` is superseded by `a-9x3b2c`, and the note's current status is `done`.
