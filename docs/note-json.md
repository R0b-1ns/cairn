# Note JSON

The JSON shapes cairn reads and writes. Used by `cairn add --stdin`, `cairn amend <id> --stdin`, `cairn import`, and printed by `--json`.

## Input rules (all JSON read from standard input or a file)

- Input is exactly one JSON object. Data before or after it is refused. A top-level array or scalar is refused. Empty input is refused.
- A UTF-8 byte order mark at the start is ignored. Surrounding whitespace is ignored.
- Unknown keys are refused at every level, with `json: unknown field`.
- Write keys exactly as named below. The decoder also accepts a key that differs only in letter case, but do not rely on that.
- A key repeated in the same object keeps its last value.
- Size: 1 MiB for `--stdin`; 32 MiB for an import file. See `limits.md`.
- CRLF: on `add` and `amend`, each `\r\n` in `title` and in entry `text` becomes `\n` before validation. On `import`, nothing is converted, so a CR in those fields is refused as a control character.

## Note document (`cairn add --stdin`)

| Name | Type | Required | Allowed | Meaning |
|---|---|---|---|---|
| `agent` | string | yes | 1-80 characters, not blank, no control characters | Name of the agent that wrote the note. |
| `repo` | string | yes | Exactly one of the project's `repos` | Repo the work was done in. |
| `task` | string | no | Empty, or a task ID matching the project's task pattern; at most 80 characters | Task the note belongs to. |
| `model` | string | yes | 1-80 characters, not blank. `unknown` is refused on add and amend (import only) | Model that did the work. |
| `title` | string | yes | 1-200 characters, not blank, one line | Short title. |
| `status` | string | yes | `done`, `in_progress`, `blocked` | Status when written. |
| `entries` | array of entry objects | yes; may be `[]` only if `none` lists all five types | 0-100 items | The content. |
| `none` | array of strings | no | Entry type names; a listed type must have no entries | Sections declared empty. |
| `impacts` | array of strings | no | Repo names from `repos`; at most 50 items | Other repos affected. |
| `commits` | array of strings | no | Lowercase hex, 7-64 characters; at most 50 items | Commit hashes. |
| `resolves` | array of strings | no | Entry IDs `e-...` of `open` entries; at most 50 items | Open entries this note closes. |
| `supersedes` | array of strings | no | Entry IDs `e-...` of `decision` entries; at most 50 items | Decisions this note replaces. |

Refused on `add`:

- `id`: assigned by cairn. Error: `id: is assigned by cairn; remove it`.
- `at`: set by cairn. Error: `at: is set by cairn; remove it`.
- `entries[i].id`: assigned by cairn.

Required on `import` in addition: `id` and `at` (RFC3339), and each `entries[i].id`.

### Valid note document

```json
{
  "agent": "builder",
  "repo": "server",
  "task": "TP-03",
  "model": "claude-opus-5-5",
  "title": "Add tidal index",
  "status": "done",
  "entries": [
    {"type": "change", "text": "Added the hourly tidal index table."},
    {"type": "decision", "by": "user", "text": "Index buckets are hourly, to match the sensor rate."},
    {"type": "open", "text": "Should old buckets be pruned after 90 days?"}
  ],
  "none": ["note", "helper"],
  "impacts": ["client"],
  "commits": ["3f9c2ab"]
}
```

Save it as `note.json` and run:

```sh
cairn add --stdin --project tidepool < note.json
```

## Addendum document (`cairn amend <note-id> --stdin`)

| Name | Type | Required | Allowed | Meaning |
|---|---|---|---|---|
| `agent` | string | yes | 1-80 characters, not blank, no control characters | Agent writing the addendum. |
| `model` | string | yes | 1-80 characters, not blank. `unknown` refused (import only) | Model that did the work. |
| `status` | string | no | `done`, `in_progress`, `blocked` | New status for the note. |
| `entries` | array of entry objects | no | 0-100 items | Content of the addendum. |
| `resolves` | array of strings | no | Entry IDs `e-...` of `open` entries; at most 50 items | Open entries, in any note, this addendum closes. |
| `supersedes` | array of strings | no | Entry IDs `e-...` of `decision` entries; at most 50 items | Decisions, in any note, this addendum replaces. |

The addendum must carry at least one of: `entries`, `status`, `resolves`, `supersedes`. Otherwise: `addendum: is empty`.

Refused on `amend`:

- `id`, `at`, and `entries[i].id`: assigned by cairn.
- `note_id`: the note is the command argument. Error: `note_id: is given as the command argument; remove it`.

Required on `import` in addition: `id`, `note_id` (a valid `n-...` ID of a note that exists or is in the same import), `at` (RFC3339), and each `entries[i].id`.

### Valid addendum document

```json
{
  "agent": "builder",
  "model": "claude-opus-5-5",
  "status": "done",
  "entries": [
    {"type": "note", "text": "Pruning runs in the nightly job."},
    {"type": "decision", "by": "agent", "text": "Keep 90 days."}
  ],
  "resolves": ["e-5w4n8p"],
  "supersedes": ["e-3d8k5r"]
}
```

Run it against note `n-4k7q2m`:

```sh
cairn amend n-4k7q2m --stdin --project tidepool < addendum.json
```

## Entry object

Entries appear inside `entries` of a note or addendum.

| Name | Type | Required | Allowed | Meaning |
|---|---|---|---|---|
| `id` | string | import only | `e-` ID, valid form | Entry ID. Refused on `add` and `amend`. |
| `type` | string | yes | `change`, `decision`, `open`, `note`, `helper` | Kind of entry. |
| `text` | string | yes | 1-4000 characters, not blank, not `none` (case-insensitive). Newline and tab allowed; other control characters refused | The text. |
| `by` | string | decisions only | `user` or `agent` | Who made the decision. Required on a `decision`; refused on any other type. |

## Export document (`cairn export --format json`, `cairn import`)

| Name | Type | Required | Allowed | Meaning |
|---|---|---|---|---|
| `format` | string | yes | Exactly `cairn-export` | Identifies the file. |
| `version` | integer | yes | Exactly `1` | Format version. |
| `project` | object | no (ignored on import) | Project object, keys below | Configuration of the exporting database. Import validates against the receiving project, not this object. |
| `notes` | array of note objects | no | Import shape above (with `id`, `at`, and entry `id`s) | Notes. |
| `addenda` | array of addendum objects | no | Import shape above (with `id`, `note_id`, `at`) | Addenda. |

Keys of the `project` object: `name`, `mode`, `context`, `repos`, `task_pattern`. Meanings are in `configuration.md`.

Import refuses a document whose `format` or `version` differs. The error is `import: not a cairn export`.

### Valid export document

```json
{
  "format": "cairn-export",
  "version": 1,
  "project": {
    "name": "tidepool",
    "mode": "project",
    "context": "Tide data service.",
    "repos": ["server", "client"],
    "task_pattern": "^(?P<base>[A-Z]{2,5}-\\d{1,4})(?:\\.(?P<ext>\\d{1,3}))?$"
  },
  "notes": [
    {
      "id": "n-4k7q2m",
      "agent": "builder",
      "repo": "server",
      "at": "2026-10-30T10:00:00Z",
      "task": "TP-03",
      "model": "claude-opus-5-5",
      "title": "Add tidal index",
      "status": "done",
      "entries": [
        {"id": "e-7h2m9q", "type": "change", "text": "Added the hourly tidal index table."},
        {"id": "e-3d8k5r", "type": "decision", "by": "user", "text": "Index buckets are hourly, to match the sensor rate."},
        {"id": "e-5w4n8p", "type": "open", "text": "Should old buckets be pruned after 90 days?"}
      ],
      "none": ["note", "helper"],
      "impacts": ["client"],
      "commits": ["3f9c2ab"]
    }
  ],
  "addenda": []
}
```

Import it with:

```sh
cairn import export.json --project tidepool
```

Import never changes existing rows. A note or addendum whose ID already exists with identical content is counted as already present. The same ID with different content refuses the whole import.
