# cairn amend

Adds an addendum to an existing note. The note itself never changes.

## Usage

```
cairn amend <note-id> --agent NAME --model MODEL [--status STATUS] [--change TEXT]... [--decision TEXT]... [--open TEXT]... [--note TEXT]... [--helper TEXT]... [--resolves ID]... [--supersedes ID]... [--project NAME] [--json]
cairn amend <note-id> --stdin [--project NAME] [--json]
```

## Arguments

| Argument | Required | Meaning |
|---|---|---|
| `<note-id>` | yes | ID of the note to add to: `n-` followed by six characters, e.g. `n-4k7q2m`. The note must exist in the project. |

## Flags

| Flag | Default | Allowed values | Meaning |
|---|---|---|---|
| `--agent` | (empty) | any name, 1-80 characters | Agent name. Required in flag mode. |
| `--model` | (empty) | any name, 1-80 characters | Model that did the work. Required in flag mode. |
| `--status` | (empty) | `done`, `in_progress`, `blocked` | New status of the note. Optional. The latest addendum that carries a status is the current status. |
| `--change` | (empty) | text | Summary of what was done. Repeatable. |
| `--decision` | (empty) | text starting with `user:` or `agent:` | A choice and its reason. Repeatable. |
| `--open` | (empty) | text | An open point. Repeatable. |
| `--note` | (empty) | text | Something worth knowing. Repeatable. |
| `--helper` | (empty) | text: path and purpose | A reusable building block. Repeatable. |
| `--resolves` | (empty) | ID of an open entry, `e-...`, in any note | Closes that open point. Repeatable. |
| `--supersedes` | (empty) | ID of a decision entry, `e-...`, in any note | Replaces that decision. Repeatable. |
| `--stdin` | `false` | flag, no value | Read the addendum as one JSON object from standard input. Only `--project` and `--json` may be combined with it. |
| `--project` | (empty) | a registered project name | Project to write to. Default: found from the current directory, or env `CAIRN_PROJECT`. |
| `--json` | `false` | flag, no value | Print the saved addendum as JSON. |

With `--stdin` the JSON object uses the keys `agent`, `model`, `status`, `entries` (objects with `type`, `text`, and `by` for decisions), `resolves`, `supersedes`. `note_id` must not be given; the note is the argument. `id` and `at` must be absent.

## Output

Text, on success:

```
addendum a-9x3b2c added to n-4k7q2m
  e-8k3m5p note
```

The second line lists each entry of the addendum by ID and type. An addendum with only `--status`, `--resolves` or `--supersedes` prints the first line only.

`--json` prints the saved addendum: `id`, `note_id`, `agent`, `model`, `at`, and `status`, `entries`, `resolves`, `supersedes` when set.

## Exit codes

- `0`: addendum saved.
- `1`: runtime error, or the note ID does not exist in the project (`note n-... not found`).
- `2`: usage error: not exactly one argument, or `--stdin` combined with an addendum flag.
- `3`: invalid input, listed on standard error. Includes an empty addendum, a missing `--agent` or `--model`, and a `--resolves` or `--supersedes` ID that does not exist.
- `4`: no project found.

## Examples

```
cairn amend n-4k7q2m --agent builder --model claude-opus-5-5 --status done --resolves e-5c8r0t --note "Pruning was added in the nightly job."
```

Prints `addendum a-9x3b2c added to n-4k7q2m` and `  e-8k3m5p note`. Exit code 0. The open point `e-5c8r0t` is now resolved.

```
cairn amend n-4k7q2m --agent builder --model claude-opus-5-5 --decision "agent:Keep 90 days of buckets."  --supersedes e-7h2m9q
```

Prints the addendum ID and one `decision` entry. The old decision `e-7h2m9q` is superseded: `cairn show` marks it as superseded.

```
cairn amend n-4k7q2m --agent builder --model claude-opus-5-5 --status blocked
```

Sets the note's current status to `blocked` with an addendum that has no entries. Exit code 0.

## Notes

- The addendum must carry at least one of: an entry, `--status`, `--resolves`, `--supersedes`. Otherwise exit code 3.
- The note ID is a positional argument. `amend` cannot create a note; use `cairn add`.
- Resolving and superseding work across notes: the target entry may belong to any note.
- A resolved open point is hidden from `list` and `brief` by default; the original text stays readable with `cairn show`.
- The note's own status never changes. `cairn show` prints the current status and, when it differs, the status set when the note was written.
